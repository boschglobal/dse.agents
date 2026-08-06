// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/boschglobal/dse.agents/mcp/pkg/registry"
	"github.com/boschglobal/dse.agents/mcp/pkg/service"
	"github.com/boschglobal/dse.agents/mcp/pkg/volume"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

type DockerRuntime struct {
	client        *client.Client
	volumeManager *volume.VolumeManager
}

func NewDockerRuntime() (*DockerRuntime, error) {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}

	runtime := &DockerRuntime{
		client: cli,
	}
	runtime.volumeManager = NewDockerVolumeManager(runtime)
	return runtime, nil
}

func (d *DockerRuntime) Run(
	ctx context.Context,
	sessionID string,
	ct *service.ContainerTask,
	spec registry.ContainerSpec,
	payload interface{},
	onProgress service.Progress,
) (service.ContainerResult, error) {
	var result service.ContainerResult
	if spec.Image == "" {
		return result, fmt.Errorf("container image is required")
	}
	if onProgress == nil {
		return result, fmt.Errorf("progress callback is required")
	}
	slog.Debug("DockerRuntime.Run()", "taskID", ct.TaskID(), "image", spec.Image)

	// Setup the container parameters.
	env := make([]string, 0, len(spec.Env))
	for k, v := range spec.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	slog.Debug("DockerRuntime.Run()", "env", env)
	if spec.VolumeGenerator != nil {
		for path, mount := range spec.VolumeGenerator(payload) {
			spec.Volume[path] = mount
		}
	}
	mounts, err := d.sessionVolumeMounts(ctx, sessionID)
	if err != nil {
		return result, fmt.Errorf("session volume mounts: %w", err)
	}

	for hostPath, containerPath := range spec.Volume {
		mounts = append(mounts, mount.Mount{
			Type: mount.TypeBind,
			Source: func() string {
				if filepath.IsAbs(hostPath) {
					return hostPath
				}
				cwd, _ := os.Getwd()
				return filepath.Join(cwd, hostPath)
			}(),
			Target: containerPath,
		})
	}
	slog.Debug("DockerRuntime.Run()", "mounts", mounts)
	entryPoint := []string{}
	if spec.UseEntrypoint {
		entryPoint = nil
	}
	slog.Debug("DockerRuntime.Run()", "entryPoint", entryPoint)
	cmd := []string{}
	if spec.CommandGenerator != nil {
		cmd = spec.CommandGenerator(payload)
	} else if spec.CommandFormatter != nil {
		command := spec.CommandFormatter(payload)
		if command != "" {
			cmd = []string{"sh", "-c", command}
		}
	}
	slog.Debug("DockerRuntime.Run()", "cmd", cmd)

	// Create the container.
	slog.Info("d.client.ContainerCreate()", "image", spec.Image)
	resp, err := d.client.ContainerCreate(
		ctx,
		&dockercontainer.Config{
			Image:        spec.Image,
			Env:          env,
			Entrypoint:   entryPoint,
			Cmd:          cmd,
			AttachStdout: true,
			AttachStderr: true,
			Tty:          false,
		},
		&dockercontainer.HostConfig{
			Mounts:     mounts,
			AutoRemove: false, // explicit cleanup in defer
		},
		nil,
		nil,
		"",
	)
	if err != nil {
		slog.Error("Container create", "resp", resp, "err", err)
		return result, fmt.Errorf("create container: %w", err)
	}
	containerID := resp.ID
	defer func() {
		_ = d.client.ContainerRemove(context.Background(), containerID, dockercontainer.RemoveOptions{
			Force: true,
		})
	}()
	onProgress(ct.TaskID(), service.Log{
		Stream:  service.LogStatus,
		Message: "container created: " + containerID,
	})

	// Start the container.
	if err := d.client.ContainerStart(ctx, containerID, dockercontainer.StartOptions{}); err != nil {
		return result, fmt.Errorf("container started: %w", err)
	}
	onProgress(ct.TaskID(), service.Log{
		Stream:  service.LogStatus,
		Message: "container started: " + containerID,
	})

	// Create a log reader for the container.
	logReader, err := d.client.ContainerLogs(ctx, containerID, dockercontainer.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	if err != nil {
		return result, fmt.Errorf("attach container logs: %w", err)
	}
	defer logReader.Close()

	// Connect the log reader and progress callbacks.
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer

	stdoutWriter := &progressWriter{
		ct:         ct,
		kind:       service.LogStdout,
		onProgress: onProgress,
		buffer:     &stdoutBuf,
	}
	stderrWriter := &progressWriter{
		ct:         ct,
		kind:       service.LogStderr,
		onProgress: onProgress,
		buffer:     &stderrBuf,
	}

	var wg sync.WaitGroup
	var logErr error

	wg.Add(1)
	go func() {
		defer wg.Done()
		_, logErr = stdcopy.StdCopy(stdoutWriter, stderrWriter, logReader)
	}()

	// Wait for the container to finish.
	statusCh, errCh := d.client.ContainerWait(ctx, containerID, dockercontainer.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return result, fmt.Errorf("wait for container: %w", err)
		}
	case status := <-statusCh:
		result.Code = status.StatusCode
	}

	wg.Wait()
	if logErr != nil && !isContextError(logErr) {
		return result, fmt.Errorf("stream container logs: %w", logErr)
	}
	//result.Stdout = stdoutBuf.String()
	//result.Stderr = stderrBuf.String()

	onProgress(ct.TaskID(), service.Log{
		Stream:  service.LogStatus,
		Message: fmt.Sprintf("container finished with exit code %d", result.Code),
	})

	return result, nil
}

func (d *DockerRuntime) VolumeExists(name string) bool {
	return d.volumeManager.VolumeExists(name)
}

func (d *DockerRuntime) sessionVolumeMounts(ctx context.Context, sessionID string) ([]mount.Mount, error) {
	var mounts []mount.Mount

	volumeList, err := d.volumeManager.ListVolumes(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	for _, vol := range volumeList {
		dockerVolumeName, err := dockerVolumeNameFromVolume(&vol)
		if err != nil {
			return nil, err
		}
		mounts = append(mounts, mount.Mount{
			Type:     mount.TypeVolume,
			Source:   dockerVolumeName,
			Target:   vol.MountPath,
			ReadOnly: vol.ReadOnly,
		})
	}
	return mounts, nil
}

func isContextError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "context canceled") || strings.Contains(s, "context deadline exceeded")
}

type progressWriter struct {
	ct         *service.ContainerTask
	kind       service.LogType
	onProgress service.Progress
	buffer     *bytes.Buffer
}

func (w *progressWriter) Write(p []byte) (int, error) {
	if w.buffer != nil {
		_, _ = w.buffer.Write(p)
	}
	log := service.Log{
		Stream:  w.kind,
		Message: string(p),
	}

	w.ct.BroadcastLog(log, w.onProgress)
	return len(p), nil
}
