// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/boschglobal/dse.agents/mcp/pkg/service"
	"github.com/boschglobal/dse.agents/mcp/pkg/task"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
)

type HealthResponse struct {
	Body struct {
		Status string `json:"status"`
	}
}

type StreamInput struct {
	ID string `path:"id" doc:"The unique task ID to stream logs from"`
}

type ResultInput struct {
	ID string `path:"id" doc:"The unique task ID to query"`
}

type CancelTaskInput struct {
	ID string `path:"id" doc:"The target task execution identifier to abort"`
}

type TaskBodyInput[T any] struct {
	Body T
}

type SSELogEvent struct {
	Stream  string `json:"stream"`
	Message string `json:"message"`
}

func RegisterHumaRoutes(api huma.API, router chi.Router, cts *service.ContainerTaskService) {
	huma.Register(api, huma.Operation{
		OperationID: "health-check",
		Method:      "GET",
		Path:        "/health",
		Summary:     "Health check",
		Description: "Returns the current health status.",
		Tags:        []string{"Utility"},
	}, func(ctx context.Context, input *struct{}) (*HealthResponse, error) {
		resp := &HealthResponse{}
		resp.Body.Status = "OK"
		return resp, nil
	})

	// Register task routes.
	registerTaskRoute[task.MessageRequest](
		api,
		cts,
		"/api/tasks/message",
		"Start message task",
		"Starts a background container task for MessageRequest.",
	)
	registerTaskRoute[task.SimerRequest](
		api,
		cts,
		"/api/tasks/simer",
		"Start simer task",
		"Starts a background container task for SimerRequest.",
	)
	registerTaskRoute[task.BuilderRequest](
		api,
		cts,
		"/api/tasks/builder",
		"Start builder task",
		"Starts a background container task for BuilderRequest.",
	)

	// Shared task lifecycle endpoints.
	registerCancelRoute(api, cts)
	registerStreamRoute(router, cts)
	registerResultRoute(api, cts)
}

func registerTaskRoute[T task.ContainerTaskRequest](
	api huma.API,
	cts *service.ContainerTaskService,
	path string,
	summary string,
	description string,
) {
	// FIXME: need a session ID, probably an endpoint to create/delete a session, and a session key to
	// authenticate/protect a session.
	type TaskResponse struct {
		Body *service.ContainerTaskResponse
	}
	huma.Register(api, huma.Operation{
		Method:        http.MethodPost,
		Path:          path,
		Summary:       summary,
		Description:   description,
		DefaultStatus: http.StatusAccepted,
	}, func(ctx context.Context, input *TaskBodyInput[T]) (*TaskResponse, error) {
		taskID, _, err := service.StartTask(
			cts,
			"FIXME-need-session",
			input.Body,
			func(taskID string, logItem service.Log) {
				// Intentionally empty here if StartTask/runContainer already fans logs out
				// through cts listeners. Keep callback available for future side effects.
			},
		)
		if err != nil {
			return nil, huma.Error500InternalServerError("Failed to start container task", err)
		}

		response := &service.ContainerTaskResponse{
			TaskID:    taskID,
			Status:    "running",
			StartedAt: time.Now().Format(time.RFC3339),
		}
		return &TaskResponse{Body: response}, nil
	})
}

func registerCancelRoute(api huma.API, cts *service.ContainerTaskService) {
	huma.Register(api, huma.Operation{
		OperationID:   "cancel-container-task",
		Method:        http.MethodDelete,
		Path:          "/api/tasks/{id}",
		Summary:       "Cancel a running container task",
		Description:   "Cancels a running background task by task ID.",
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, input *CancelTaskInput) (*struct{}, error) {
		if ok := cts.CancelTask(input.ID); !ok {
			return nil, huma.Error404NotFound("Target task ID not found or already completed", nil)
		}
		return nil, nil
	})
}

func registerStreamRoute(router chi.Router, cts *service.ContainerTaskService) {
	router.Get("/api/tasks/{id}/stream", func(w http.ResponseWriter, r *http.Request) {
		taskID := chi.URLParam(r, "id")
		if taskID == "" {
			http.Error(w, "missing task id", http.StatusBadRequest)
			return
		}

		logChan, err := cts.SubscribeToLogs(taskID)
		if err != nil {
			http.Error(w, "cannot stream logs, task not found", http.StatusNotFound)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		ctx := r.Context()

		for {
			select {
			case <-ctx.Done():
				return

			case logItem, open := <-logChan:
				if !open {
					_, _ = fmt.Fprint(w, "event: end\ndata: Task finished\n\n")
					flusher.Flush()
					return
				}

				payload, err := json.Marshal(SSELogEvent{
					Stream:  string(logItem.Stream),
					Message: logItem.Message,
				})
				if err != nil {
					http.Error(w, "failed to encode SSE payload", http.StatusInternalServerError)
					return
				}

				_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
				flusher.Flush()
			}
		}
	})
}

func registerResultRoute(api huma.API, cts *service.ContainerTaskService) {
	type ResultResponse struct {
		Body *service.ContainerResult
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-container-task-result",
		Method:      http.MethodGet,
		Path:        "/api/tasks/{id}",
		Summary:     "Get task status or final result",
		Description: "Returns the current or final status/result for a task.",
	}, func(ctx context.Context, input *ResultInput) (*ResultResponse, error) {
		result, err := cts.GetTaskResult(input.ID)
		if err != nil {
			return nil, huma.Error404NotFound("Task not found", err)
		}
		return &ResultResponse{Body: result}, nil
	})
}
