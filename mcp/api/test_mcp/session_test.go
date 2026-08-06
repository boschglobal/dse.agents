// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package test_mcp

import (
	"fmt"
	"testing"
	"time"

	"github.com/boschglobal/dse.agents/mcp/pkg/session"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPServer_Session_Hooks(t *testing.T) {
	f := newMCPStdioFixture(t, "mock", 10*time.Minute, 5*time.Minute)

	f.Initialize(t)
	activeSessions := f.WaitForSessionCount(t, 1)
	activeSessionID := activeSessions[0].ID
	require.Equal(t, activeSessionID, "stdio")

	f.Close(t)
	_, stillExists := f.sm.GetSession(activeSessionID)
	require.False(t, stillExists)
}

func TestMCPServer_Session_RunTask(t *testing.T) {
	f := newMCPStdioFixture(t, "mock", 10*time.Minute, 5*time.Minute)

	f.Initialize(t)
	activeSessions := f.WaitForSessionCount(t, 1)
	activeSessionID := activeSessions[0].ID
	require.Equal(t, activeSessionID, "stdio")

	// Run Task
	createResult, err := f.CreateTaskFromTool(t, "echo_message", map[string]interface{}{
		"message": "Session message",
	})
	require.NoError(t, err)
	require.NotNil(t, createResult)
	require.NotEmpty(t, createResult.Task.TaskId)

	taskResult, err := f.TaskResult(t, createResult.Task.TaskId)
	require.NoError(t, err)
	require.NotNil(t, taskResult)
	require.False(t, taskResult.IsError, "task result returned tool error payload: %v", taskResult.Content)
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		f.notificationsMu.Lock()
		notifications := append([]mcp.JSONRPCNotification(nil), f.notifications...)
		f.notificationsMu.Unlock()
		require.Contains(c, fmt.Sprint(notifications), "mock echo_message received: Session message")
	},
		1*time.Second,
		10*time.Millisecond,
	)
	userSession, sessionExists := f.sm.GetSession(activeSessionID)
	require.True(t, sessionExists, "Session %s no longer exists after task execution", activeSessionID)
	require.Equal(
		t,
		session.StateIdle,
		userSession.State(),
		"State Machine Fault: Session lock remained on StateRunningTask upon task completion, found: %v",
		userSession.State(),
	)

	// Run Task when session is busy.
	_, err = userSession.Acquire()
	require.NoError(t, err)
	defer func() {
		_, _ = userSession.Release()
	}()

	createResult, err = f.CreateTaskFromTool(t, "echo_message", map[string]interface{}{
		"message": "Second session message",
	})
	require.NoError(t, err)
	require.NotNil(t, createResult)
	require.NotEmpty(t, createResult.Task.TaskId)

	taskResult, err = f.TaskResult(t, createResult.Task.TaskId)
	require.Error(t, err)
	require.ErrorContains(t, err, "rejected: session in use")
	require.Nil(t, taskResult)

	// Closing stdio client.
	f.Close(t)
	_, stillExists := f.sm.GetSession(activeSessionID)
	require.False(t, stillExists)
}

func TestMCPServer_Session_Volume(t *testing.T) {
	f := newMCPStdioFixture(t, "mock", 10*time.Minute, 5*time.Minute)

	f.Initialize(t)
	activeSessions := f.WaitForSessionCount(t, 1)
	activeSessionID := activeSessions[0].ID
	require.Equal(t, "stdio", activeSessionID)

	createWorkspaceVolume(t, f, "workspace", "memfs", "/workspace")

	createAndRequireTaskSuccess(t, f, "write_volume_file", map[string]interface{}{
		"volume": "workspace",
		"path":   "hello.txt",
		"data":   "hello world from memfs volume integration test\n",
	})

	createAndRequireTaskSuccess(t, f, "list_volume_files", map[string]interface{}{
		"volume": "workspace",
		"path":   ".",
	})
	requireProgressContains(t, f, "hello.txt")

	createAndRequireTaskSuccess(t, f, "read_volume_file", map[string]interface{}{
		"volume": "workspace",
		"path":   "hello.txt",
	})
	requireProgressContains(t, f, "hello world from memfs volume integration test")

	createAndRequireTaskSuccess(t, f, "delete_volume", map[string]interface{}{
		"volume": "workspace",
	})

	userSession, sessionExists := f.sm.GetSession(activeSessionID)
	require.True(t, sessionExists, "Session %s no longer exists after volume task execution", activeSessionID)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		_, volumeExists := userSession.GetVolume("workspace")
		require.False(c, volumeExists, "volume workspace still exists after delete_volume task")
	},
		1*time.Second,
		10*time.Millisecond,
	)

	require.Equal(
		t,
		session.StateIdle,
		userSession.State(),
		"State Machine Fault: Session lock remained on StateRunningTask upon volume task completion, found: %v",
		userSession.State(),
	)

	f.Close(t)
	_, stillExists := f.sm.GetSession(activeSessionID)
	require.False(t, stillExists)
}

func TestMCPServer_Session_CloseDeletesVolumes(t *testing.T) {
	f := newMCPStdioFixture(t, "mock", 10*time.Minute, 5*time.Minute)

	f.Initialize(t)
	activeSessions := f.WaitForSessionCount(t, 1)
	activeSessionID := activeSessions[0].ID
	require.Equal(t, "stdio", activeSessionID)

	createWorkspaceVolume(t, f, "workspace", "memfs", "/workspace")

	userSession, sessionExists := f.sm.GetSession(activeSessionID)
	require.True(t, sessionExists)
	_, volumeExists := userSession.GetVolume("workspace")
	require.True(t, volumeExists, "volume workspace should exist before closing session")

	f.Close(t)

	requireSessionAndRuntimeVolumeDeleted(t, f, activeSessionID, "workspace")
}

func TestMCPServer_Session_Reaper(t *testing.T) {
	f := newMCPStdioFixture(t, "mock", 10*time.Minute, 10*time.Millisecond)

	f.Initialize(t)
	activeSessions := f.WaitForSessionCount(t, 1)
	activeSessionID := activeSessions[0].ID
	require.Equal(t, "stdio", activeSessionID)

	createWorkspaceVolume(t, f, "workspace", "memfs", "/workspace")

	userSession, sessionExists := f.sm.GetSession(activeSessionID)
	require.True(t, sessionExists, "session %s should exist before reaper runs", activeSessionID)
	_, volumeExists := userSession.GetVolume("workspace")
	require.True(t, volumeExists, "volume workspace should exist before reaper runs")
	requireRuntimeVolumeExists(t, *f.rt, "workspace")

	require.Equal(
		t,
		session.StateIdle,
		userSession.State(),
		"session must be idle before reaper can collect it")
	userSession.SetLastActivity(time.Now().Add(-1 * time.Hour))
	requireSessionAndRuntimeVolumeDeleted(t, f, activeSessionID, "workspace")
}

func createWorkspaceVolume(t *testing.T, f *mcpStdioFixture, name string, kind string, mount string) {
	t.Helper()

	createAndRequireTaskSuccess(t, f, "create_volume", map[string]interface{}{
		"name":      name,
		"kind":      kind,
		"mountPath": mount,
		"readOnly":  false,
	})
}

func createAndRequireTaskSuccess(
	t *testing.T,
	f *mcpStdioFixture,
	toolName string,
	args map[string]interface{},
) *mcp.TaskResultResult {
	t.Helper()

	activeSessions := f.WaitForSessionCount(t, 1)
	require.NotEmpty(t, activeSessions)
	requireSessionIdle(t, f, activeSessions[0].ID)

	createResult, err := f.CreateTaskFromTool(t, toolName, args)
	require.NoError(t, err)
	require.NotNil(t, createResult)
	require.NotEmpty(t, createResult.Task.TaskId)

	var taskResult *mcp.TaskResultResult
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		var resultErr error
		taskResult, resultErr = f.TaskResult(t, createResult.Task.TaskId)
		require.NoError(c, resultErr)
		require.NotNil(c, taskResult)
		require.False(c, taskResult.IsError, "task result returned tool error payload: %v", taskResult.Content)
	},
		1*time.Second,
		10*time.Millisecond,
	)

	return taskResult
}
