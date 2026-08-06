// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package mock

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/boschglobal/dse.agents/mcp/pkg/registry"
	"github.com/boschglobal/dse.agents/mcp/pkg/service"
	"github.com/boschglobal/dse.agents/mcp/pkg/volume"
)

type MockRuntime struct {
	volumeManager *volume.VolumeManager
}

func NewMockRuntime() *MockRuntime {
	return &MockRuntime{
		volumeManager: NewMockVolumeManager(),
	}
}

func (m *MockRuntime) Run(ctx context.Context, sessionID string, ct *service.ContainerTask, spec registry.ContainerSpec,
	payload interface{}, onProgress service.Progress,
) (service.ContainerResult, error) {
	switch spec.Image {
	case "dse-message:test":
		return m.runMessageMock(ct, spec, payload, onProgress)
	}

	onProgress(ct.TaskID(), service.Log{
		Stream:  service.LogStatus,
		Message: "running mock service.Container task (not the requested task!)",
	})
	onProgress(ct.TaskID(), service.Log{
		Stream:  service.LogStatus,
		Message: fmt.Sprintf("container.Container finished with exit code %d", 0),
	})
	return service.ContainerResult{Code: 0}, nil
}

func (m *MockRuntime) runMessageMock(
	ct *service.ContainerTask,
	spec registry.ContainerSpec,
	payload interface{},
	onProgress service.Progress,
) (service.ContainerResult, error) {
	message := extractMockMessage(payload)

	onProgress(ct.TaskID(), service.Log{
		Stream:  service.LogStatus,
		Message: "running mock echo_message service.Container task",
	})
	onProgress(ct.TaskID(), service.Log{
		Stream:  service.LogStatus,
		Message: fmt.Sprintf("mock echo_message received: %s", message),
	})
	onProgress(ct.TaskID(), service.Log{
		Stream:  service.LogStatus,
		Message: fmt.Sprintf("container.Container finished with exit code %d", 0),
	})
	return service.ContainerResult{Code: 0}, nil
}

func extractMockMessage(payload interface{}) string {
	if args, ok := payload.(map[string]any); ok {
		if message, ok := args["message"].(string); ok {
			return message
		}
	}

	var decoded struct {
		Message string `json:"message"`
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprint(payload)
	}

	if err := json.Unmarshal(b, &decoded); err != nil {
		return fmt.Sprint(payload)
	}

	if decoded.Message == "" {
		return fmt.Sprint(payload)
	}

	return decoded.Message
}

func (m *MockRuntime) VolumeExists(name string) bool {
	return m.volumeManager.VolumeExists(name)
}
