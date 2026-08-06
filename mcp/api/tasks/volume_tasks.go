// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package tasks

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/boschglobal/dse.agents/mcp/pkg/service"
	"github.com/boschglobal/dse.agents/mcp/pkg/session"
	"github.com/boschglobal/dse.agents/mcp/pkg/task"
	"github.com/boschglobal/dse.agents/mcp/pkg/volume"
	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func encodeNoContent(struct{}) ([]mcp.Content, error) {
	return nil, nil
}

func registerTaskCreateVolume(
	s *server.MCPServer,
	vts *service.VolumeTaskService,
	sm *session.Manager,
) {
	registerAsyncVolumeTask(
		s,
		vts,
		sm,
		"create_volume",
		"Creates a volume for the current session.",
		func(
			ctx context.Context,
			vts *service.VolumeTaskService,
			managedSession *session.Session,
			input task.CreateVolumeRequest,
			onProgress service.TaskProgress,
		) (struct{}, error) {
			_, err := vts.CreateVolume(ctx, managedSession, input, onProgress)
			return struct{}{}, err
		},
		encodeNoContent,
	)
}

func registerTaskWriteVolumeFile(
	s *server.MCPServer,
	vts *service.VolumeTaskService,
	sm *session.Manager,
) {
	registerAsyncVolumeTask(
		s,
		vts,
		sm,
		"write_volume_file",
		"Writes a file into a volume for the current session.",
		func(
			ctx context.Context,
			vts *service.VolumeTaskService,
			managedSession *session.Session,
			input task.WriteVolumeFileRequest,
			onProgress service.TaskProgress,
		) (struct{}, error) {
			return struct{}{}, vts.WriteVolumeFile(ctx, managedSession, input, onProgress)
		},
		encodeNoContent,
	)
}

func registerTaskListVolumeFiles(
	s *server.MCPServer,
	vts *service.VolumeTaskService,
	sm *session.Manager,
) {
	registerAsyncVolumeTask(
		s,
		vts,
		sm,
		"list_volume_files",
		"Lists all files contained in the path of the specified volume for the current session.",
		func(
			ctx context.Context,
			vts *service.VolumeTaskService,
			managedSession *session.Session,
			input task.ListVolumeFilesRequest,
			onProgress service.TaskProgress,
		) ([]volume.FileInfo, error) {
			return vts.ListVolumeFiles(ctx, managedSession, input, onProgress)
		},
		encodeFileList,
	)
}

func encodeFileList(files []volume.FileInfo) ([]mcp.Content, error) {
	if len(files) == 0 {
		return []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: "no files",
			},
		}, nil
	}

	lines := make([]string, 0, len(files))
	for _, file := range files {
		if file.IsDir {
			lines = append(lines, file.Path+"/")
		} else {
			lines = append(lines, file.Path)
		}
	}

	return []mcp.Content{
		mcp.TextContent{
			Type: "text",
			Text: strings.Join(lines, "\n"),
		},
	}, nil
}

func registerTaskReadVolumeFile(
	s *server.MCPServer,
	vts *service.VolumeTaskService,
	sm *session.Manager,
) {
	registerAsyncVolumeTask(
		s,
		vts,
		sm,
		"read_volume_file",
		"Reads a file from a volume for the current session.",
		func(
			ctx context.Context,
			vts *service.VolumeTaskService,
			managedSession *session.Session,
			input task.ReadVolumeFileRequest,
			onProgress service.TaskProgress,
		) ([]byte, error) {
			return vts.ReadVolumeFile(ctx, managedSession, input, onProgress)
		},
		encodeFileContent,
	)
}

func encodeFileContent(data []byte) ([]mcp.Content, error) {
	if utf8.Valid(data) {
		return []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: string(data),
			},
		}, nil
	}

	encoded := struct {
		Encoding string `json:"encoding"`
		Data     string `json:"data"`
		Size     int    `json:"size"`
	}{
		Encoding: "base64",
		Data:     base64.StdEncoding.EncodeToString(data),
		Size:     len(data),
	}

	jsonData, err := json.MarshalIndent(encoded, "", "  ")
	if err != nil {
		return nil, err
	}

	return []mcp.Content{
		mcp.TextContent{
			Type: "text",
			Text: string(jsonData),
		},
	}, nil
}

func registerTaskDeleteVolume(
	s *server.MCPServer,
	vts *service.VolumeTaskService,
	sm *session.Manager,
) {
	registerAsyncVolumeTask(
		s,
		vts,
		sm,
		"delete_volume",
		"Delete a volume from the current session.",
		func(
			ctx context.Context,
			vts *service.VolumeTaskService,
			managedSession *session.Session,
			input task.DeleteVolumeRequest,
			onProgress service.TaskProgress,
		) (struct{}, error) {
			return struct{}{}, vts.DeleteVolume(ctx, managedSession, input, onProgress)
		},
		encodeNoContent,
	)
}

func registerAsyncVolumeTask[T any, PT interface {
	*T
	taskIDSetter
}, R any](
	s *server.MCPServer,
	vts *service.VolumeTaskService,
	sm *session.Manager,
	name string,
	description string,
	run func(
		ctx context.Context,
		vts *service.VolumeTaskService,
		managedSession *session.Session,
		input T,
		onProgress service.TaskProgress,
	) (R, error),
	encodeResult func(R) ([]mcp.Content, error),
) {
	fieldOpts, err := buildToolOptions[T]()
	if err != nil {
		panic(err)
	}

	opts := []mcp.ToolOption{
		mcp.WithDescription(description),
		mcp.WithTaskSupport(mcp.TaskSupportOptional),
	}
	opts = append(opts, fieldOpts...)

	tool := mcp.NewTool(name, opts...)

	s.AddTaskTool(tool, withAsyncSession(sm, func(
		ctx context.Context,
		r mcp.CallToolRequest,
		managedSession *session.Session,
	) (*mcp.CreateTaskResult, error) {
		clientSession := server.ClientSessionFromContext(ctx)
		if clientSession == nil {
			return nil, fmt.Errorf("protocol failure: request has no session")
		}

		input, err := parseArguments[T](r)
		if err != nil {
			return nil, err
		}

		taskID := uuid.New().String()
		//input.SetTaskID(taskID)
		PT(&input).SetTaskID(taskID)

		onProgress := service.TaskProgress(func(taskID string, message string) {
			progressParams := map[string]interface{}{
				"progressToken": taskID,
				"meta": map[string]interface{}{
					"status": message,
				},
			}

			sessionCtx := s.WithContext(context.Background(), clientSession)

			if err := s.SendNotificationToClient(
				sessionCtx,
				"notifications/progress",
				progressParams,
			); err != nil {
				slog.Error("SendNotificationToClient failed", "err", err)
			}
		})

		go func() {
			defer managedSession.Release()

			taskCtx := context.Background()

			result, err := run(taskCtx, vts, managedSession, input, onProgress)
			if err != nil {
				onProgress.Report(
					taskID,
					fmt.Sprintf("%s failed: %v", name, err),
				)
				return
			}

			// TODO: implement taskStore for results so that final TaskResult
			// can be constructed.
			content, err := encodeResult(result)
			if err != nil {
				onProgress.Report(
					taskID,
					fmt.Sprintf("%s result encoding failed: %v", name, err),
				)
				return
			}

			reportContentAsProgress(taskID, content, onProgress)
			onProgress.Report(
				taskID,
				fmt.Sprintf("%s completed", name),
			)
		}()

		return createTaskResult(
			taskID,
			fmt.Sprintf("%s started", name),
			nil,
		), nil
	}))
}

func reportContentAsProgress(
	taskID string,
	content []mcp.Content,
	onProgress service.TaskProgress,
) {
	for _, item := range content {
		switch c := item.(type) {
		case mcp.TextContent:
			if c.Text != "" {
				onProgress.Report(taskID, c.Text)
			}

		case *mcp.TextContent:
			if c != nil && c.Text != "" {
				onProgress.Report(taskID, c.Text)
			}

		default:
			onProgress.Report(taskID, fmt.Sprint(c))
		}
	}
}
