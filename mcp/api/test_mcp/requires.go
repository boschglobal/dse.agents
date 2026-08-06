// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package test_mcp

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/boschglobal/dse.agents/mcp/pkg/service"
	"github.com/boschglobal/dse.agents/mcp/pkg/session"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireDockerAvailable(t *testing.T) {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping Docker integration test in short mode")
	}

	if _, err := exec.LookPath("docker"); err != nil {
		t.Skipf("skipping Docker integration test: docker CLI not found: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "info")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("skipping Docker integration test: Docker daemon unavailable: %v\n%s", err, string(output))
	}
}

func requireSessionIdle(t *testing.T, f *mcpStdioFixture, sessionID string) {
	t.Helper()

	require.Eventually(t, func() bool {
		userSession, ok := f.sm.GetSession(sessionID)
		if !ok {
			return false
		}
		return userSession.State() == session.StateIdle
	}, 5*time.Second, 10*time.Millisecond, "session %s did not become idle", sessionID)
}

type runtimeVolumeInspector interface {
	VolumeExists(name string) bool
}

func requireRuntimeVolumeExists(
	t *testing.T,
	rt service.Runtime,
	name string,
) {
	t.Helper()
	inspector, ok := rt.(runtimeVolumeInspector)
	require.True(t, ok, "runtime does not support volume inspection")
	require.True(t, inspector.VolumeExists(name), "runtime volume %s should exist", name)
}

func requireRuntimeVolumeDeleted(
	t *testing.T,
	rt service.Runtime,
	name string,
) {
	t.Helper()

	inspector, ok := rt.(runtimeVolumeInspector)
	require.True(t, ok, "runtime does not support volume inspection")
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		require.False(c, inspector.VolumeExists(name), "runtime volume %s still exists", name)
	},
		1*time.Second,
		10*time.Millisecond,
	)
}

func requireProgressContains(
	t *testing.T,
	f *mcpStdioFixture,
	expected string,
) {
	t.Helper()

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		f.notificationsMu.Lock()
		notifications := append([]mcp.JSONRPCNotification(nil), f.notifications...)
		f.notificationsMu.Unlock()

		require.Contains(c, fmt.Sprint(notifications), expected)
	},
		1*time.Second,
		10*time.Millisecond,
	)
}

func requireSessionAndRuntimeVolumeDeleted(
	t *testing.T,
	f *mcpStdioFixture,
	sessionID string,
	volumeName string,
) {
	t.Helper()

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		_, stillExists := f.sm.GetSession(sessionID)
		require.False(c, stillExists, "session %s still exists", sessionID)
	},
		1*time.Second,
		10*time.Millisecond,
	)
	requireRuntimeVolumeDeleted(t, *f.rt, volumeName)
}
