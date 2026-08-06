// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"context"

	"github.com/boschglobal/dse.agents/mcp/pkg/runtime/mock"
	"github.com/boschglobal/dse.agents/mcp/pkg/service"
	"github.com/boschglobal/dse.agents/mcp/pkg/volume"
)

func NewDockerVolumeManager(runtime *DockerRuntime) *volume.VolumeManager {
	return volume.NewVolumeManager(map[volume.Kind]volume.VolumeProvider{
		volume.KindMemFS:     mock.NewMemFSProvider(),
		volume.KindHostPath:  NewHostPathProvider(runtime),
		volume.KindTmpFS:     NewTmpfsProvider(runtime),
		volume.KindEphemeral: NewEphemeralProvider(runtime),
	})
}

func (d *DockerRuntime) CreateVolume(
	ctx context.Context,
	sessionID string,
	spec volume.Spec,
	onProgress service.Progress,
) (*volume.Volume, error) {
	return d.volumeManager.CreateVolume(ctx, sessionID, spec)
}

func (d *DockerRuntime) DeleteVolume(
	ctx context.Context,
	sessionID string,
	vol *volume.Volume,
	onProgress service.Progress,
) error {
	return d.volumeManager.DeleteVolume(ctx, sessionID, vol)
}

func (d *DockerRuntime) ListVolumeFiles(
	ctx context.Context,
	sessionID string,
	vol *volume.Volume,
	path string,
) ([]volume.FileInfo, error) {
	return d.volumeManager.ListFiles(ctx, sessionID, vol, path)
}

func (d *DockerRuntime) ReadVolumeFile(
	ctx context.Context,
	sessionID string,
	vol *volume.Volume,
	path string,
) ([]byte, error) {
	return d.volumeManager.ReadFile(ctx, sessionID, vol, path)
}

func (d *DockerRuntime) WriteVolumeFile(
	ctx context.Context,
	sessionID string,
	vol *volume.Volume,
	path string,
	data []byte,
) error {
	return d.volumeManager.WriteFile(ctx, sessionID, vol, path, data)
}
