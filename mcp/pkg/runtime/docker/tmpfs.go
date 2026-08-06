// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"context"
	"fmt"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	dockervolume "github.com/docker/docker/api/types/volume"

	"github.com/boschglobal/dse.agents/mcp/pkg/volume"
)

type TmpfsHandle struct {
	DockerVolumeName  string
	KeeperContainerID string
}

func (h TmpfsHandle) DockerVolume() string {
	return h.DockerVolumeName
}

type TmpfsProvider struct {
	*Provider
}

func NewTmpfsProvider(runtime *DockerRuntime) *TmpfsProvider {
	return &TmpfsProvider{
		Provider: NewProvider(runtime, ProviderConfig{
			LogName:       "tmpfs",
			VolumeOptions: tmpfsVolumeOptions,
			PostCreate:    startTmpfsKeeperContainer,
			PreDelete:     deleteTmpfsKeeperContainer,
		}),
	}
}

const defaultTmpfsSize = "8m"

func tmpfsVolumeOptions(
	ctx context.Context,
	sessionID string,
	spec volume.Spec,
) (dockervolume.CreateOptions, DockerVolumeHandle, error) {
	size := defaultTmpfsSize
	if spec.TmpFS != nil && spec.TmpFS.Size != "" {
		size = spec.TmpFS.Size
	}

	dockerVolumeName := fmt.Sprintf("%s_%s", sessionID, spec.Name)
	handle := TmpfsHandle{
		DockerVolumeName: dockerVolumeName,
	}

	return dockervolume.CreateOptions{
		Name:   dockerVolumeName,
		Driver: "local",
		DriverOpts: map[string]string{
			"type":   "tmpfs",
			"device": "tmpfs",
			"o":      fmt.Sprintf("size=%s", size),
		},
		Labels: map[string]string{
			"dse.session": sessionID,
			"dse.kind":    string(spec.Kind),
			"dse.name":    spec.Name,
			"dse.volume":  dockerVolumeName,
		},
	}, handle, nil
}

func startTmpfsKeeperContainer(ctx context.Context, p *Provider, vol *volume.Volume) error {
	handle, ok := vol.Handle.(TmpfsHandle)
	if !ok {
		return fmt.Errorf("volume %q has unexpected tmpfs handle type %T", vol.Name, vol.Handle)
	}

	containerID, err := p.createVolumeHelperContainer(
		ctx,
		handle.DockerVolumeName,
		false,
		[]string{"sleep", "infinity"},
	)
	if err != nil {
		return err
	}

	if err := p.runtime.client.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		_ = p.runtime.client.ContainerRemove(ctx, containerID, container.RemoveOptions{
			Force:         true,
			RemoveVolumes: false,
		})
		return fmt.Errorf("start tmpfs keeper container %q: %w", containerID, err)
	}

	vol.Handle = TmpfsHandle{
		DockerVolumeName:  handle.DockerVolumeName,
		KeeperContainerID: containerID,
	}

	return nil
}

func deleteTmpfsKeeperContainer(ctx context.Context, p *Provider, vol *volume.Volume) error {
	handle, ok := vol.Handle.(TmpfsHandle)
	if !ok {
		return fmt.Errorf("volume %q has unexpected tmpfs handle type %T", vol.Name, vol.Handle)
	}

	if handle.KeeperContainerID == "" {
		return nil
	}

	if err := p.runtime.client.ContainerRemove(ctx, handle.KeeperContainerID, container.RemoveOptions{
		Force:         true,
		RemoveVolumes: false,
	}); err != nil && !cerrdefs.IsNotFound(err) {
		return fmt.Errorf("delete tmpfs keeper container %q: %w", handle.KeeperContainerID, err)
	}

	return nil
}
