// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package mock

import (
	"context"

	"github.com/boschglobal/dse.agents/mcp/pkg/service"
	"github.com/boschglobal/dse.agents/mcp/pkg/volume"
)

func NewMockVolumeManager() *volume.VolumeManager {
	return volume.NewVolumeManager(map[volume.Kind]volume.VolumeProvider{
		volume.KindMemFS: NewMemFSProvider(),
	})
}

func (m *MockRuntime) CreateVolume(
	ctx context.Context,
	sessionID string,
	spec volume.Spec,
	onProgress service.Progress,
) (*volume.Volume, error) {
	return m.volumeManager.CreateVolume(ctx, sessionID, spec)
}

func (m *MockRuntime) DeleteVolume(
	ctx context.Context,
	sessionID string,
	vol *volume.Volume,
	onProgress service.Progress,
) error {
	return m.volumeManager.DeleteVolume(ctx, sessionID, vol)
}

func (m *MockRuntime) ListVolumeFiles(
	ctx context.Context,
	sessionID string,
	vol *volume.Volume,
	path string,
) ([]volume.FileInfo, error) {
	return m.volumeManager.ListFiles(ctx, sessionID, vol, path)
}

func (m *MockRuntime) ReadVolumeFile(
	ctx context.Context,
	sessionID string,
	vol *volume.Volume,
	path string,
) ([]byte, error) {
	return m.volumeManager.ReadFile(ctx, sessionID, vol, path)
}

func (m *MockRuntime) WriteVolumeFile(
	ctx context.Context,
	sessionID string,
	vol *volume.Volume,
	path string,
	data []byte,
) error {
	return m.volumeManager.WriteFile(ctx, sessionID, vol, path, data)
}
