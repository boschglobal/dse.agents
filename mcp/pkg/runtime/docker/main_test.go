// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"bytes"
	"context"
	"log/slog"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"go.uber.org/goleak"
)

type Suite struct {
	suite.Suite
	logBuffer *bytes.Buffer
	oldLogger *slog.Logger
}

func (s *Suite) BeforeTest(suiteName, testName string) {
	s.logBuffer = new(bytes.Buffer)
	s.oldLogger = slog.Default()

	opts := &slog.HandlerOptions{Level: slog.LevelDebug}
	logger := slog.New(slog.NewTextHandler(s.logBuffer, opts))
	slog.SetDefault(logger)
}

func (s *Suite) AfterTest(suiteName, testName string) {
	if s.T().Failed() {
		s.T().Logf("\n--- CAPTURED DEBUG LOGS FOR %s ---\n%s", testName, s.logBuffer.String())
	}
	if s.oldLogger != nil {
		slog.SetDefault(s.oldLogger)
	}
	s.logBuffer.Reset()
}

func TestRuntimeDocker(t *testing.T) {
	requireDockerAvailable(t)

	suite.Run(t, new(Suite))
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(
		m,
		goleak.IgnoreAnyFunction("net/http.(*persistConn).readLoop"),
		goleak.IgnoreAnyFunction("net/http.(*persistConn).writeLoop"),
	)
}

func newIntegrationDockerRuntime(t *testing.T, ctx context.Context) *DockerRuntime {
	t.Helper()

	rt, err := NewDockerRuntime()
	if err != nil {
		t.Skipf("Docker runtime unavailable: %v", err)
	}

	if _, err := rt.client.Ping(ctx); err != nil {
		t.Skipf("Docker daemon unavailable: %v", err)
	}

	return rt
}

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
