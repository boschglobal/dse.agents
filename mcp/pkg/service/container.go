// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/boschglobal/dse.agents/mcp/pkg/registry"
	"github.com/boschglobal/dse.agents/mcp/pkg/task"
	"github.com/google/uuid"
)

type ContainerTaskService struct {
	activeTasks sync.Map // taskID -> *ContainerTask
	results     sync.Map // taskID -> *ContainerResult
	runtime     Runtime
	resultTTL   time.Duration
}

type ContainerTask struct {
	taskID    string
	mu        sync.Mutex
	cancel    context.CancelFunc
	listeners []chan Log
	logs      []Log
	startedAt time.Time
	done      bool
}

func (ct *ContainerTask) TaskID() string {
	return ct.taskID
}

func (ct *ContainerTask) BroadcastLog(log Log, onProgress Progress) {
	ct.mu.Lock()
	ct.logs = append(ct.logs, log)
	listeners := append([]chan Log(nil), ct.listeners...)
	taskID := ct.taskID
	ct.mu.Unlock()

	if onProgress != nil {
		onProgress(taskID, log)
	}

	for _, ch := range listeners {
		select {
		case ch <- log:
		default:
		}
	}
}

// Response to task request, generic, async task.
type ContainerTaskResponse struct {
	TaskID    string `json:"task_id"    doc:"Unique tracker ID to manage or stream logs"`
	Status    string `json:"status"     doc:"Initial execution state"`
	StartedAt string `json:"started_at" doc:"RFC3339 initialization timestamp"`
}

// Final task status ... needed?? when task finishes.
type ContainerResult struct {
	TaskID     string `json:"task_id"`
	Status     string `json:"status"                doc:"Can be 'running', 'completed', or 'failed'"`
	Code       int64  `json:"code"                  doc:"The final exit code of the Docker container process"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at,omitempty"`
}

func NewContainerTaskService(ctx context.Context, driver Runtime) (*ContainerTaskService, error) {
	cts := &ContainerTaskService{
		runtime: driver,
	}
	go cts.startCleanupLoop(ctx)
	return cts, nil
}

func RunTask[T task.ContainerTaskRequest](
	cts *ContainerTaskService,
	sessionID string,
	request T,
	onProgress Progress,
) (string, error) {
	requestType := reflect.TypeOf(request)
	def, exists := registry.Registry[requestType]
	if !exists {
		return "", fmt.Errorf("unregistered task request: %s", requestType)
	}

	taskID := uuid.New().String()
	ctx, cancel := context.WithCancel(context.Background())
	task := &ContainerTask{taskID: taskID, cancel: cancel}
	cts.activeTasks.Store(taskID, task)
	defer cts.activeTasks.Delete(taskID)
	err := cts.runContainer(ctx, sessionID, taskID, def, request, onProgress)

	return taskID, err
}

func StartTask[T task.ContainerTaskRequest](
	cts *ContainerTaskService,
	sessionID string,
	request T,
	onProgress Progress,
) (string, context.CancelFunc, error) {
	requestType := reflect.TypeOf(request)
	def, exists := registry.Registry[requestType]
	if !exists {
		return "", nil, fmt.Errorf("unregistered task request: %s", requestType)
	}

	taskID := uuid.New().String()
	ctx, cancel := context.WithCancel(context.Background())
	task := &ContainerTask{taskID: taskID, cancel: cancel}
	cts.activeTasks.Store(taskID, task)
	go func() {
		defer cts.activeTasks.Delete(taskID)
		_ = cts.runContainer(ctx, sessionID, taskID, def, request, onProgress)
	}()

	return taskID, cancel, nil
}

func (cts *ContainerTaskService) CancelTask(taskID string) bool {
	if v, ok := cts.activeTasks.Load(taskID); ok {
		ct := v.(*ContainerTask)
		ct.mu.Lock()
		cancel := ct.cancel
		ct.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		cts.finishTask(ct)
		cts.activeTasks.Delete(taskID)
		return true

		// TODO stop the container ... note use the generalised interface.
	}
	return false
}

func (cts *ContainerTaskService) SubscribeToLogs(taskID string) (chan Log, error) {
	val, ok := cts.activeTasks.Load(taskID)
	if !ok {
		return nil, errors.New("task not found or already completed")
	}

	ct := val.(*ContainerTask)
	ch := make(chan Log, 10)

	ct.mu.Lock()
	backlog := append([]Log(nil), ct.logs...)
	done := ct.done
	if !done {
		ct.listeners = append(ct.listeners, ch)
	}
	ct.mu.Unlock()

	go func() {
		for _, log := range backlog {
			ch <- log
		}
		if done {
			close(ch)
		}
	}()

	return ch, nil
}

func (cts *ContainerTaskService) GetTaskResult(taskID string) (*ContainerResult, error) {
	if v, ok := cts.results.Load(taskID); ok {
		return v.(*ContainerResult), nil
	}

	if v, ok := cts.activeTasks.Load(taskID); ok {
		ct := v.(*ContainerTask)
		ct.mu.Lock()
		defer ct.mu.Unlock()

		return &ContainerResult{
			TaskID:    taskID,
			Status:    "running",
			StartedAt: ct.startedAt.Format(time.RFC3339),
		}, nil
	}

	return nil, fmt.Errorf("task not found")
}

func (cts *ContainerTaskService) startCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			cts.results.Range(func(key, value any) bool {
				res := value.(*ContainerResult)
				finishedAt, err := time.Parse(time.RFC3339, res.FinishedAt)
				if err == nil && now.Sub(finishedAt) > cts.resultTTL {
					cts.results.Delete(key)
				}
				return true
			})
		}
	}
}

func (cts *ContainerTaskService) runContainer(
	ctx context.Context,
	sessionID string,
	taskID string,
	spec registry.ContainerSpec, request interface{}, onProgress Progress) error {
	v, exists := cts.activeTasks.Load(taskID)
	if !exists {
		return fmt.Errorf("task not in list of active tasks: %v", taskID)
	}
	ct := v.(*ContainerTask)

	result, err := cts.runtime.Run(ctx, sessionID, ct, spec, request, onProgress)
	result.StartedAt = ct.startedAt.Format(time.RFC3339)
	result.FinishedAt = time.Now().Format(time.RFC3339)
	cts.results.Store(taskID, result)
	cts.finishTask(ct)
	cts.activeTasks.Delete((taskID))
	return err
}

func (cts *ContainerTaskService) finishTask(ct *ContainerTask) {
	ct.mu.Lock()
	ct.done = true
	listeners := append([]chan Log(nil), ct.listeners...)
	ct.listeners = nil
	ct.mu.Unlock()

	for _, ch := range listeners {
		close(ch)
	}
}
