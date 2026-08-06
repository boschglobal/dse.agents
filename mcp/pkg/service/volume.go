// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"

	"github.com/boschglobal/dse.agents/mcp/pkg/session"
	"github.com/boschglobal/dse.agents/mcp/pkg/task"
	"github.com/boschglobal/dse.agents/mcp/pkg/volume"
)

type VolumeTaskService struct {
	runtime Runtime
}

func NewVolumeTaskService(runtime Runtime) *VolumeTaskService {
	return &VolumeTaskService{
		runtime: runtime,
	}
}

func (s *VolumeTaskService) CreateVolume(
	ctx context.Context,
	clientSession *session.Session,
	input task.CreateVolumeRequest,
	onProgress TaskProgress,
) (*volume.Volume, error) {
	onProgress.Report(input.TaskID, "creating volume")

	runtimeProgress := Progress(func(taskID string, logEntry Log) {
		onProgress.Report(taskID, logEntry.Message)
	})

	vol, err := s.runtime.CreateVolume(
		ctx,
		clientSession.ID,
		volume.Spec{
			Name:      input.Name,
			Kind:      input.Kind,
			MountPath: input.MountPath,
			ReadOnly:  input.ReadOnly,
		},
		runtimeProgress,
	)
	if err != nil {
		return nil, err
	}

	if err := clientSession.AddVolume(vol); err != nil {
		if deleteErr := s.runtime.DeleteVolume(
			ctx,
			clientSession.ID,
			vol,
			runtimeProgress,
		); deleteErr != nil {
			return nil, fmt.Errorf(
				"add volume %q to session: %w (rollback deletion failed: %v)",
				vol.Name,
				err,
				deleteErr,
			)
		}
		return nil, fmt.Errorf("add volume %q to session: %w", vol.Name, err)
	}

	onProgress.Report(input.TaskID, fmt.Sprintf("volume %q created", vol.Name))
	return vol, nil
}

func (s *VolumeTaskService) WriteVolumeFile(
	ctx context.Context,
	managedSession *session.Session,
	input task.WriteVolumeFileRequest,
	onProgress TaskProgress,
) error {
	if input.Volume == "" {
		return fmt.Errorf("volume is required")
	}
	path := input.Path
	if path == "" {
		path = "."
	}
	data := []byte(input.Data)

	vol, ok := managedSession.GetVolume(input.Volume)
	if !ok {
		return fmt.Errorf("volume %q not found in session %q", input.Volume, managedSession.ID)
	}

	onProgress.Report(
		input.TaskID,
		fmt.Sprintf("writing file in volume %q at %q", input.Volume, path),
	)
	err := s.runtime.WriteVolumeFile(
		ctx,
		managedSession.ID,
		vol,
		path,
		data,
	)
	if err != nil {
		return err
	}
	onProgress.Report(
		input.TaskID,
		fmt.Sprintf("file written to volume %q with path %q", input.Volume, path),
	)
	return nil
}

func (s *VolumeTaskService) ListVolumeFiles(
	ctx context.Context,
	managedSession *session.Session,
	input task.ListVolumeFilesRequest,
	onProgress TaskProgress,
) ([]volume.FileInfo, error) {
	if input.Volume == "" {
		return nil, fmt.Errorf("volume is required")
	}
	path := input.Path
	if path == "" {
		path = "."
	}
	vol, ok := managedSession.GetVolume(input.Volume)
	if !ok {
		return nil, fmt.Errorf("volume %q not found in session %q", input.Volume, managedSession.ID)
	}

	onProgress.Report(
		input.TaskID,
		fmt.Sprintf("listing files in volume %q at %q", input.Volume, path),
	)
	files, err := s.runtime.ListVolumeFiles(
		ctx,
		managedSession.ID,
		vol,
		path,
	)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		onProgress.Report(input.TaskID, "no files")
		return files, nil
	}
	for _, file := range files {
		onProgress.Report(input.TaskID, file.Path)
	}
	return files, nil
}

func (s *VolumeTaskService) ReadVolumeFile(
	ctx context.Context,
	managedSession *session.Session,
	input task.ReadVolumeFileRequest,
	onProgress TaskProgress,
) ([]byte, error) {
	if input.Volume == "" {
		return nil, fmt.Errorf("volume is required")
	}
	if input.Path == "" {
		return nil, fmt.Errorf("path is required")
	}
	vol, ok := managedSession.GetVolume(input.Volume)
	if !ok {
		return nil, fmt.Errorf("volume %q not found in session %q", input.Volume, managedSession.ID)
	}

	onProgress.Report(
		input.TaskID,
		fmt.Sprintf("reading file from volume %q at path %q", input.Volume, input.Path),
	)
	data, err := s.runtime.ReadVolumeFile(
		ctx,
		managedSession.ID,
		vol,
		input.Path,
	)
	if err != nil {
		return nil, err
	}
	onProgress.Report(
		input.TaskID,
		fmt.Sprintf("read %d bytes from volume %q at path %q", len(data), input.Volume, input.Path),
	)
	return data, nil
}

func (s *VolumeTaskService) DeleteVolume(
	ctx context.Context,
	managedSession *session.Session,
	input task.DeleteVolumeRequest,
	onProgress TaskProgress,
) error {
	if input.Volume == "" {
		return fmt.Errorf("volume is required")
	}
	vol, ok := managedSession.GetVolume(input.Volume)
	if !ok {
		return fmt.Errorf("volume %q not found in session %q", input.Volume, managedSession.ID)
	}

	onProgress.Report(
		input.TaskID,
		fmt.Sprintf("deleting volume %q", input.Volume),
	)
	runtimeProgress := Progress(func(taskID string, logEntry Log) {
		onProgress.Report(taskID, logEntry.Message)
	})
	if err := s.runtime.DeleteVolume(
		ctx,
		managedSession.ID,
		vol,
		runtimeProgress,
	); err != nil {
		return err
	}
	managedSession.RemoveVolume(input.Volume)
	onProgress.Report(
		input.TaskID,
		fmt.Sprintf("volume %q deleted", input.Volume),
	)
	return nil
}
