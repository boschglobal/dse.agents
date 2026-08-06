// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package volume

import (
	"context"
	"fmt"
)

type VolumeProvider interface {
	ListVolumes(ctx context.Context, sessionID string) ([]Volume, error)
	CreateVolume(ctx context.Context, sessionID string, spec Spec) (*Volume, error)
	DeleteVolume(ctx context.Context, sessionID string, vol *Volume) error
	WriteVolumeFile(ctx context.Context, sessionID string, vol *Volume, filePath string, data []byte) error
	ReadVolumeFile(ctx context.Context, sessionID string, vol *Volume, filePath string) ([]byte, error)
	ListVolumeFiles(ctx context.Context, sessionID string, vol *Volume, dirPath string) ([]FileInfo, error)
	VolumeExists(name string) bool
}

type VolumeManager struct {
	providers map[Kind]VolumeProvider
}

func NewVolumeManager(providers map[Kind]VolumeProvider) *VolumeManager {
	return &VolumeManager{
		providers: providers,
	}
}

func (m *VolumeManager) ListVolumes(ctx context.Context, sessionID string) ([]Volume, error) {
	volumes := []Volume{}
	for kind, provider := range m.providers {
		providerVolumes, err := provider.ListVolumes(ctx, sessionID)
		if err != nil {
			return nil, fmt.Errorf("list %s volumes: %w", kind, err)
		}
		volumes = append(volumes, providerVolumes...)
	}
	return volumes, nil
}

func (m *VolumeManager) CreateVolume(ctx context.Context, sessionID string, spec Spec) (*Volume, error) {
	provider, exists := m.providers[spec.Kind]
	if !exists {
		return nil, fmt.Errorf("unsupported volume kind: %s", spec.Kind)
	}
	return provider.CreateVolume(ctx, sessionID, spec)
}

func (m *VolumeManager) DeleteVolume(ctx context.Context, sessionID string, vol *Volume) error {
	provider, exists := m.providers[vol.Kind]
	if !exists {
		return fmt.Errorf("unsupported volume kind: %s", vol.Kind)
	}
	return provider.DeleteVolume(ctx, sessionID, vol)
}

func (m *VolumeManager) WriteFile(
	ctx context.Context,
	sessionID string,
	vol *Volume,
	filePath string,
	data []byte,
) error {
	provider, exists := m.providers[vol.Kind]
	if !exists {
		return fmt.Errorf("unsupported volume kind: %s", vol.Kind)
	}
	return provider.WriteVolumeFile(ctx, sessionID, vol, filePath, data)
}

func (m *VolumeManager) ReadFile(ctx context.Context, sessionID string, vol *Volume, filePath string) ([]byte, error) {
	provider, exists := m.providers[vol.Kind]
	if !exists {
		return nil, fmt.Errorf("unsupported volume kind: %s", vol.Kind)
	}
	return provider.ReadVolumeFile(ctx, sessionID, vol, filePath)
}

func (m *VolumeManager) ListFiles(
	ctx context.Context,
	sessionID string,
	vol *Volume,
	dirPath string,
) ([]FileInfo, error) {
	provider, exists := m.providers[vol.Kind]
	if !exists {
		return nil, fmt.Errorf("unsupported volume kind: %s", vol.Kind)
	}
	return provider.ListVolumeFiles(ctx, sessionID, vol, dirPath)
}

func (m *VolumeManager) VolumeExists(name string) bool {
	for _, provider := range m.providers {
		if provider.VolumeExists(name) {
			return true
		}
	}
	return false
}
