// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package tasks

import (
	"context"
	"fmt"

	"github.com/boschglobal/dse.agents/mcp/pkg/service"
	"github.com/boschglobal/dse.agents/mcp/pkg/session"
	"github.com/boschglobal/dse.agents/mcp/pkg/task"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerTaskMessage(s *server.MCPServer, cts *service.ContainerTaskService, sm *session.Manager) {
	fieldOpts, err := buildToolOptions[task.MessageRequest]()
	if err != nil {
		panic(err)
	}
	opts := []mcp.ToolOption{
		mcp.WithDescription("Prints a message to the console."),
		mcp.WithTaskSupport(mcp.TaskSupportOptional),
	}
	opts = append(opts, fieldOpts...)

	messageTool := mcp.NewTool("echo_message", opts...)
	s.AddTaskTool(
		messageTool,
		withSession(sm, func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CreateTaskResult, error) {
			return runTask(
				ctx,
				s,
				r,
				parseArguments[task.MessageRequest],
				func(sessionID string, input task.MessageRequest, onProgress service.Progress) (string, error) {
					return service.RunTask(cts, sessionID, input, onProgress)
				},
				func(input task.MessageRequest) string {
					return fmt.Sprintf("message processed: %s", input.Message)
				},
				"echo_message failed",
			)
		}),
	)
}

func registerTaskFileprint(s *server.MCPServer, cts *service.ContainerTaskService, sm *session.Manager) {
	fieldOpts, err := buildToolOptions[task.FileprintRequest]()
	if err != nil {
		panic(err)
	}
	opts := []mcp.ToolOption{
		mcp.WithDescription("Prints the specified file to the console."),
		mcp.WithTaskSupport(mcp.TaskSupportOptional),
	}
	opts = append(opts, fieldOpts...)

	fileprintTool := mcp.NewTool("file_print", opts...)
	s.AddTaskTool(
		fileprintTool,
		withSession(sm, func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CreateTaskResult, error) {
			return runTask(
				ctx,
				s,
				r,
				parseArguments[task.FileprintRequest],
				func(sessionID string, input task.FileprintRequest, onProgress service.Progress) (string, error) {
					return service.RunTask(cts, sessionID, input, onProgress)
				},
				func(input task.FileprintRequest) string {
					return fmt.Sprintf("file printed: %s", input.Path)
				},
				"file_print failed",
			)
		}),
	)
}

func registerTaskSimer(s *server.MCPServer, cts *service.ContainerTaskService, sm *session.Manager) {
	fieldOpts, err := buildToolOptions[task.SimerRequest]()
	if err != nil {
		panic(err)
	}
	opts := []mcp.ToolOption{
		mcp.WithDescription("Runs a simulation in the Simer container."),
		mcp.WithTaskSupport(mcp.TaskSupportOptional),
	}
	opts = append(opts, fieldOpts...)

	simerTool := mcp.NewTool("run_simer", opts...)
	s.AddTaskTool(
		simerTool,
		withSession(sm, func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CreateTaskResult, error) {
			return runTask(
				ctx,
				s,
				r,
				parseArguments[task.SimerRequest],
				func(sessionID string, input task.SimerRequest, onProgress service.Progress) (string, error) {
					return service.RunTask(cts, sessionID, input, onProgress)
				},
				func(input task.SimerRequest) string {
					return fmt.Sprintf("run_simer completed: %s", input.SimPath)
				},
				"run_simer failed",
			)
		}),
	)
}
