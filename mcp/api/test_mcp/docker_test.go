// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package test_mcp

import (
	"testing"
	"time"

	"github.com/boschglobal/dse.agents/mcp/pkg/session"
	"github.com/stretchr/testify/require"
)

func TestMCPServer_Docker_TaskVolume_Tmpfs(t *testing.T) {
	requireDockerAvailable(t)

	f := newMCPStdioFixture(t, "docker", 10*time.Minute, 5*time.Minute)

	f.Initialize(t)
	activeSessions := f.WaitForSessionCount(t, 1)
	activeSessionID := activeSessions[0].ID
	require.Equal(t, "stdio", activeSessionID)

	createWorkspaceVolume(t, f, "workspace", "tmpfs", "/workspace")

	createAndRequireTaskSuccess(t, f, "write_volume_file", map[string]interface{}{
		"volume": "workspace",
		"path":   "hello.txt",
		"data":   "hello world from docker task volume test\n",
	})

	createAndRequireTaskSuccess(t, f, "file_print", map[string]interface{}{
		"path": "/workspace/hello.txt",
	})

	requireProgressContains(t, f, "hello world from docker task volume test")

	userSession, sessionExists := f.sm.GetSession(activeSessionID)
	require.True(t, sessionExists, "Session %s no longer exists after docker task volume execution", activeSessionID)
	require.Equal(
		t,
		session.StateIdle,
		userSession.State(),
		"State Machine Fault: Session lock remained on StateRunningTask upon docker task volume completion, found: %v",
		userSession.State(),
	)

	f.Close(t)
	requireSessionAndRuntimeVolumeDeleted(t, f, activeSessionID, "workspace")
}
