// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"log"
	"log/slog"

	"github.com/boschglobal/dse.agents/mcp/api/tasks"
	"github.com/boschglobal/dse.agents/mcp/pkg/service"
	"github.com/boschglobal/dse.agents/mcp/pkg/session"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Server - the server itself, operating on stdout
// Resources - things like files, static or dynamic (generated)
// Tools - similar to POST endpoints
// Tasks - asynchronous tools. Polling interface
//		TaskSupportForbidden
//		TaskSupportOptional
//		TaskSupportRequired

func CreateMcpServer(
	sm *session.Manager,
	cts *service.ContainerTaskService,
	vts *service.VolumeTaskService,
) (*server.MCPServer, error) {
	hooks := &server.Hooks{}
	hooks.AddOnRegisterSession(func(ctx context.Context, clientSession server.ClientSession) {
		sessionID := clientSession.SessionID()
		s := sm.CreateSession(sessionID)
		log.Printf("Session register: SessionID=%s, s.ID=%s", sessionID, s.ID)
	})
	hooks.AddOnUnregisterSession(func(ctx context.Context, clientSession server.ClientSession) {
		sessionID := clientSession.SessionID()
		log.Printf("Session unregister: SessionID=%s", sessionID)
		if err := sm.DestroySession(ctx, sessionID); err != nil {
			log.Printf("Warning: error during call to DestroySession (%v)", err)
		}
	})
	hooks.AddBeforeAny(func(ctx context.Context, id any, method mcp.MCPMethod, message any) {
		log.Printf("Processing %s request", method)
	})
	hooks.AddOnError(func(ctx context.Context, id any, method mcp.MCPMethod, message any, err error) {
		log.Printf("Error in %s: %v", method, err)
	})

	taskHooks := &server.TaskHooks{}
	taskHooks.AddOnTaskCreated(func(ctx context.Context, metrics server.TaskMetrics) {
		log.Printf("Task %s created for tool %s", metrics.TaskID, metrics.ToolName)
	})
	taskHooks.AddOnTaskCompleted(func(ctx context.Context, metrics server.TaskMetrics) {
		log.Printf("Task %s completed in %v", metrics.TaskID, metrics.Duration)
	})
	taskHooks.AddOnTaskFailed(func(ctx context.Context, metrics server.TaskMetrics) {
		log.Printf("Task %s failed: %v", metrics.TaskID, metrics.Error)
	})
	taskHooks.AddOnTaskCancelled(func(ctx context.Context, metrics server.TaskMetrics) {
		log.Printf("Task %s was cancelled", metrics.TaskID)
		cts.CancelTask(metrics.TaskID)
	})
	taskHooks.AddOnTaskStatusChanged(func(ctx context.Context, metrics server.TaskMetrics) {
		log.Printf("Task %s status: %s", metrics.TaskID, metrics.Status)
	})

	slog.Debug("API:MCP: new MCP server")
	s := server.NewMCPServer(
		"DSE Agent",
		"0.1.0", // TODO: get version from build infra.

		server.WithRecovery(),
		server.WithToolCapabilities(true),
		server.WithTaskCapabilities(true, true, true),
		server.WithMaxConcurrentTasks(4),
		server.WithHooks(hooks),
		server.WithTaskHooks(taskHooks),
	)

	tasks.RegisterTasks(s, cts, vts, sm)
	return s, nil
}
