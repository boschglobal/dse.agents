// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package task

import "github.com/boschglobal/dse.agents/mcp/pkg/volume"

type CreateVolumeRequest struct {
	TaskID    string      `json:"taskId"    jsonschema:"Task identifier used for progress notifications."`
	Name      string      `json:"name"      jsonschema:"Name of the volume."`
	Kind      volume.Kind `json:"kind"      jsonschema:"Volume kind."`
	MountPath string      `json:"mountPath" jsonschema:"Mount path of the volume."`
	ReadOnly  bool        `json:"readOnly"  jsonschema:"Whether the volume is read-only."`
}

type WriteVolumeFileRequest struct {
	TaskID string `json:"taskId,omitempty"`
	Volume string `json:"volume"           jsonschema:"required,description=Name of the volume"`
	Path   string `json:"path"             jsonschema:"required,description=Path inside the volume"`
	Data   string `json:"data"             jsonschema:"required,description=File content to write"`
}

type ReadVolumeFileRequest struct {
	TaskID string `json:"taskId,omitempty"`
	Volume string `json:"volume"           jsonschema:"required,description=Name of the volume"`
	Path   string `json:"path"             jsonschema:"required,description=Path inside the volume"`
}

type ListVolumeFilesRequest struct {
	TaskID string `json:"taskId,omitempty"`
	Volume string `json:"volume"           jsonschema:"required,description=Name of the volume"`
	Path   string `json:"path,omitempty"   jsonschema:"description=Directory path inside the volume"`
}

type DeleteVolumeRequest struct {
	TaskID string `json:"taskId,omitempty"`
	Volume string `json:"volume"           jsonschema:"required,description=Name of the volume"`
}

func (r *CreateVolumeRequest) SetTaskID(taskID string) {
	r.TaskID = taskID
}

func (r *WriteVolumeFileRequest) SetTaskID(taskID string) {
	r.TaskID = taskID
}

func (r *ListVolumeFilesRequest) SetTaskID(taskID string) {
	r.TaskID = taskID
}

func (r *ReadVolumeFileRequest) SetTaskID(taskID string) {
	r.TaskID = taskID
}

func (r *DeleteVolumeRequest) SetTaskID(taskID string) {
	r.TaskID = taskID
}
