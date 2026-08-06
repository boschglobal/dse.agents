// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package test_mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/boschglobal/dse.agents/mcp/api"
	"github.com/boschglobal/dse.agents/mcp/pkg/runtime"
	"github.com/boschglobal/dse.agents/mcp/pkg/service"
	"github.com/boschglobal/dse.agents/mcp/pkg/session"
	"github.com/boschglobal/dse.agents/mcp/pkg/volume"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type mcpStdioFixture struct {
	ctx             context.Context
	cancel          context.CancelFunc
	sm              *session.Manager
	rt              *service.Runtime
	cts             *service.ContainerTaskService
	mcpServer       *server.MCPServer
	mcpClient       *client.Client
	serverDone      chan error
	cancelServer    context.CancelFunc
	closeOnce       sync.Once
	notificationsMu sync.Mutex
	notifications   []mcp.JSONRPCNotification
}

func newMCPStdioFixture(t *testing.T, runtimeName string, idle time.Duration, scan time.Duration) *mcpStdioFixture {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())

	// Session manager
	sm := session.NewManager(ctx, idle, scan)

	// Container task service
	var rt service.Runtime
	var cts *service.ContainerTaskService
	var vts *service.VolumeTaskService
	var err error
	rt, err = runtime.NewRuntime(runtimeName)
	if err != nil {
		cancel()
		t.Fatalf("failed to get runtime: %v", err)
	}
	cts, err = service.NewContainerTaskService(ctx, rt)
	if err != nil {
		cancel()
		t.Fatalf("failed to boot container task service: %v", err)
	}
	vts = service.NewVolumeTaskService(rt)

	sm.SetVolumeTeardown(func(ctx context.Context, sessionID string, vol *volume.Volume) error {
		return rt.DeleteVolume(
			ctx,
			sessionID,
			vol,
			service.Progress(func(taskID string, log service.Log) {
				slog.Debug("Session: volume cleanup progress", "sessionID", sessionID, "vol.Name", vol.Name)
			}))
	})

	// MCP server
	mcpServer, err := api.CreateMcpServer(sm, cts, vts)
	if err != nil {
		cancel()
		t.Fatalf("failed to build MCP server instance: %v", err)
	}

	// Stdio wiring
	serverStdin, clientStdout := io.Pipe()
	clientStdin, serverStdout := io.Pipe()
	serverCtx, cancelServer := context.WithCancel(ctx)
	stdioServer := server.NewStdioServer(mcpServer)
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- stdioServer.Listen(serverCtx, serverStdin, serverStdout)
	}()

	// Client setup
	stdioTransport := transport.NewIO(
		clientStdin,
		clientStdout,
		io.NopCloser(strings.NewReader("")),
	)
	mcpClient := client.NewClient(stdioTransport)
	if err := mcpClient.Start(ctx); err != nil {
		cancelServer()
		cancel()
		t.Fatalf("failed to start stdio MCP client: %v", err)
	}

	// Complete the fixture
	f := &mcpStdioFixture{
		ctx:          ctx,
		cancel:       cancel,
		sm:           sm,
		rt:           &rt,
		cts:          cts,
		mcpServer:    mcpServer,
		mcpClient:    mcpClient,
		serverDone:   serverDone,
		cancelServer: cancelServer,
	}
	mcpClient.OnNotification(func(notification mcp.JSONRPCNotification) {
		f.notificationsMu.Lock()
		f.notifications = append(f.notifications, notification)
		f.notificationsMu.Unlock()

		params, err := json.MarshalIndent(notification.Params, "", "  ")
		if err != nil {
			t.Logf("MCP notification: method=%q params=%v", notification.Method, notification.Params)
			return
		}

		t.Logf("MCP notification: method=%q params=%s", notification.Method, string(params))
	})
	t.Cleanup(func() {
		f.Close(t)
	})
	return f
}

func (f *mcpStdioFixture) Initialize(t *testing.T) {
	t.Helper()

	initCtx, cancel := context.WithTimeout(f.ctx, 5*time.Second)
	defer cancel()

	_, err := f.mcpClient.Initialize(initCtx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "test-client",
				Version: "1.0.0",
			},
			Capabilities: mcp.ClientCapabilities{
				Tasks: &mcp.TasksCapability{},
			},
		},
	})
	if err != nil {
		t.Fatalf("stdio JSON-RPC handshake initialization failed: %v", err)
	}
}

func (f *mcpStdioFixture) WaitForSessionCount(t *testing.T, want int) []*session.Session {
	t.Helper()

	deadline := time.Now().Add(1 * time.Second)
	var sessions []*session.Session

	for time.Now().Before(deadline) {
		sessions = f.sm.GetAllSessions()
		if len(sessions) == want {
			return sessions
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("unexpected session count (%d), wanted %d", len(sessions), want)
	return nil
}

func (f *mcpStdioFixture) CallTool(
	t *testing.T,
	name string,
	toolArgs map[string]interface{},
) (*mcp.CallToolResult, error) {
	t.Helper()

	callCtx, cancel := context.WithTimeout(f.ctx, 10*time.Second)
	defer cancel()

	resp, err := f.mcpClient.CallTool(callCtx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: toolArgs,
		},
	})

	return resp, err
}

func (f *mcpStdioFixture) CreateTaskFromTool(
	t *testing.T,
	name string,
	toolArgs map[string]interface{},
) (*mcp.CreateTaskResult, error) {
	t.Helper()

	callCtx, cancel := context.WithTimeout(f.ctx, 10*time.Second)
	defer cancel()

	ttlMs := int64(60_000)
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: toolArgs,
			Task: &mcp.TaskParams{
				TTL: &ttlMs,
			},
		},
	}
	response, err := f.mcpClient.GetTransport().SendRequest(callCtx, transport.JSONRPCRequest{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      mcp.NewRequestId(int64(1)),
		Method:  string(mcp.MethodToolsCall),
		Params:  req.Params,
	})
	if err != nil {
		return nil, err
	}

	var result mcp.CreateTaskResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal CreateTaskResult: %w", err)
	}

	return &result, nil
}

func (f *mcpStdioFixture) TaskResult(
	t *testing.T,
	taskID string,
) (*mcp.TaskResultResult, error) {
	t.Helper()

	resultCtx, cancel := context.WithTimeout(f.ctx, 10*time.Second)
	defer cancel()

	return f.mcpClient.TaskResult(resultCtx, mcp.TaskResultRequest{
		Params: mcp.TaskResultParams{
			TaskId: taskID,
		},
	})
}

func (f *mcpStdioFixture) Close(t *testing.T) {
	t.Helper()

	f.closeOnce.Do(func() {
		if f.mcpClient != nil {
			_ = f.mcpClient.Close()
			f.mcpClient = nil
		}

		select {
		case err := <-f.serverDone:
			if err != nil && !errors.Is(err, context.Canceled) {
				t.Fatalf("stdio server exited with error: %v", err)
			}
		case <-time.After(1 * time.Second):
			if f.cancelServer != nil {
				f.cancelServer()
			}
			t.Fatal("stdio server did not shut down after client close")
		}
		if f.cancelServer != nil {
			f.cancelServer()
			f.cancelServer = nil
		}
		if f.cancel != nil {
			f.cancel()
			f.cancel = nil
		}
	})
}
