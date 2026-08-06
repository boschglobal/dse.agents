// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"context"
	"fmt"

	dockervolume "github.com/docker/docker/api/types/volume"

	"github.com/boschglobal/dse.agents/mcp/pkg/volume"
)

type EphemeralHandle struct {
	DockerVolumeName string
}

func (h EphemeralHandle) DockerVolume() string {
	return h.DockerVolumeName
}

type EphemeralProvider struct {
	*Provider
}

func NewEphemeralProvider(runtime *DockerRuntime) *EphemeralProvider {
	return &EphemeralProvider{
		Provider: NewProvider(runtime, ProviderConfig{
			LogName:       "ephemeral",
			VolumeOptions: ephemeralVolumeOptions,
		}),
	}
}

func ephemeralVolumeOptions(
	ctx context.Context,
	sessionID string,
	spec volume.Spec,
) (dockervolume.CreateOptions, DockerVolumeHandle, error) {
	dockerVolumeName := fmt.Sprintf("%s_%s", sessionID, spec.Name)
	handle := EphemeralHandle{
		DockerVolumeName: dockerVolumeName,
	}

	return dockervolume.CreateOptions{
		Name:   dockerVolumeName,
		Driver: "local",
		Labels: map[string]string{
			"dse.session": sessionID,
			"dse.kind":    string(spec.Kind),
			"dse.name":    spec.Name,
			"dse.volume":  dockerVolumeName,
		},
	}, handle, nil
}
