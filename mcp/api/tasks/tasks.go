// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	"github.com/boschglobal/dse.agents/mcp/pkg/service"
	"github.com/boschglobal/dse.agents/mcp/pkg/session"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type TaskRunnerFunc func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CreateTaskResult, error)

type AsyncTaskRunnerFunc func(
	ctx context.Context,
	r mcp.CallToolRequest,
	managedSession *session.Session,
) (*mcp.CreateTaskResult, error)

type taskIDSetter interface {
	SetTaskID(string)
}

func RegisterTasks(
	s *server.MCPServer,
	cts *service.ContainerTaskService,
	vts *service.VolumeTaskService,
	sm *session.Manager,
) {
	/* Container Task service. */
	registerTaskMessage(s, cts, sm)
	registerTaskFileprint(s, cts, sm)
	registerTaskSimer(s, cts, sm)

	/* Volume service. */
	registerTaskCreateVolume(s, vts, sm)
	registerTaskWriteVolumeFile(s, vts, sm)
	registerTaskListVolumeFiles(s, vts, sm)
	registerTaskReadVolumeFile(s, vts, sm)
	registerTaskDeleteVolume(s, vts, sm)
}

func createTaskResult(taskID string, message string, err error) *mcp.CreateTaskResult {
	if err != nil {
		return &mcp.CreateTaskResult{
			Task: mcp.Task{},
			Content: []mcp.Content{
				mcp.NewTextContent(message),
			},
			StructuredContent: map[string]any{
				"ok":     false,
				"taskID": taskID,
				"error":  err.Error(),
			},
			IsError: true,
		}
	}

	return &mcp.CreateTaskResult{
		Task: mcp.Task{},
		Content: []mcp.Content{
			mcp.NewTextContent(message),
		},
		StructuredContent: map[string]any{
			"ok":     true,
			"taskID": taskID,
		},
		IsError: false,
	}
}

// func runTask[T task.ContainerTaskRequest](
func runTask[T any](
	ctx context.Context,
	s *server.MCPServer,
	r mcp.CallToolRequest,
	parse func(mcp.CallToolRequest) (T, error),
	exec func(sessionID string, input T, onProgress service.Progress) (string, error),
	successMessage func(T) string,
	errorPrefix string,
) (*mcp.CreateTaskResult, error) {
	session := server.ClientSessionFromContext(ctx)
	if session == nil {
		return nil, fmt.Errorf("no active session")
	}
	sessionID := session.SessionID()

	input, err := parse(r)
	if err != nil {
		return nil, err
	}

	onProgress := service.Progress(func(taskID string, logEntry service.Log) {
		progressParams := map[string]interface{}{
			"progressToken": taskID,
			"meta": map[string]interface{}{
				string(logEntry.Stream): logEntry.Message,
			},
		}

		sessionCtx := s.WithContext(context.Background(), session)

		slog.Info("SendNotificationToClient", "stream", logEntry.Stream, "message", logEntry.Message)
		if err := s.SendNotificationToClient(sessionCtx, "notifications/progress", progressParams); err != nil {
			slog.Error("SendNotificationToClient failed", "err", err)
		}
	})

	taskID, err := exec(sessionID, input, onProgress)
	if err != nil {
		return createTaskResult(taskID, fmt.Sprintf("%s: %v", errorPrefix, err), err), nil
	}
	return createTaskResult(taskID, successMessage(input), nil), nil
}

func buildToolOptions[T any]() ([]mcp.ToolOption, error) {
	var zero T
	typ := reflect.TypeOf(zero)
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct type")
	}

	var opts []mcp.ToolOption
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if !f.IsExported() {
			continue
		}
		name := strings.Split(f.Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			continue
		}

		// Extract the tags.
		desc := f.Tag.Get("doc")
		required := strings.Contains(f.Tag.Get("validate"), "required")
		propOpts := []mcp.PropertyOption{}
		if desc != "" {
			propOpts = append(propOpts, mcp.Description(desc))
		}
		if required {
			propOpts = append(propOpts, mcp.Required())
		}

		// Determine the type.
		switch {
		case f.Type.Kind() == reflect.Bool:
			opts = append(opts, mcp.WithBoolean(name, propOpts...))
		case f.Type.Kind() == reflect.String:
			opts = append(opts, mcp.WithString(name, propOpts...))
		case f.Type.Kind() == reflect.Int || f.Type.Kind() == reflect.Float64:
			opts = append(opts, mcp.WithNumber(name, propOpts...))
		case f.Type.Kind() == reflect.Slice && f.Type.Elem().Kind() == reflect.String:
			opts = append(opts, mcp.WithArray(name, propOpts...))
		case f.Type.Kind() == reflect.Map &&
			f.Type.Key().Kind() == reflect.String &&
			f.Type.Elem().Kind() == reflect.String:
			opts = append(opts, mcp.WithObject(name, propOpts...))
		default:
			return nil, fmt.Errorf("unsupported field type %s for %s", f.Type, f.Name)
		}
	}
	return opts, nil
}

func parseArguments[T any](r mcp.CallToolRequest) (T, error) {
	var input T

	bytes, err := json.Marshal(r.Params.Arguments)
	if err != nil {
		return input, err
	}
	if err := json.Unmarshal(bytes, &input); err != nil {
		return input, err
	}

	return input, nil
}

func withSession(sm *session.Manager, task TaskRunnerFunc) server.TaskToolHandlerFunc {
	return server.TaskToolHandlerFunc(func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CreateTaskResult, error) {
		clientSession := server.ClientSessionFromContext(ctx)
		if clientSession == nil {
			return nil, fmt.Errorf("protocol failure: request has no session")
		}
		s := sm.CreateSession(clientSession.SessionID())
		if _, err := s.Acquire(); err != nil {
			return nil, err
		}
		defer func() {
			s.Release()
		}()
		return task(ctx, r)
	})
}

func withAsyncSession(
	sm *session.Manager,
	task AsyncTaskRunnerFunc,
) server.TaskToolHandlerFunc {
	return server.TaskToolHandlerFunc(func(
		ctx context.Context,
		r mcp.CallToolRequest,
	) (*mcp.CreateTaskResult, error) {
		clientSession := server.ClientSessionFromContext(ctx)
		if clientSession == nil {
			return nil, fmt.Errorf("protocol failure: request has no session")
		}

		managedSession := sm.CreateSession(clientSession.SessionID())
		if _, err := managedSession.Acquire(); err != nil {
			return nil, err
		}

		result, err := task(ctx, r, managedSession)
		if err != nil {
			managedSession.Release()
			return nil, err
		}

		// The async task now owns the acquired session reference.
		return result, nil
	})
}
