// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	dockervolume "github.com/docker/docker/api/types/volume"

	"github.com/boschglobal/dse.agents/mcp/pkg/volume"
)

func (h HostPathHandle) DockerVolume() string {
	return h.DockerVolumeName
}

type HostPathProvider struct {
	*Provider
}

type HostPathHandle struct {
	HostPath         string
	DockerVolumeName string
}

func NewHostPathProvider(runtime *DockerRuntime) *HostPathProvider {
	return &HostPathProvider{
		Provider: NewProvider(runtime, ProviderConfig{
			LogName:       "hostpath",
			VolumeOptions: hostPathVolumeOptions,
		}),
	}
}

func hostPathVolumeOptions(
	ctx context.Context,
	sessionID string,
	spec volume.Spec,
) (dockervolume.CreateOptions, DockerVolumeHandle, error) {
	if spec.HostPath == nil {
		return dockervolume.CreateOptions{}, nil, fmt.Errorf("hostPath config is required for hostPath volume")
	}
	if spec.HostPath.Path == "" {
		return dockervolume.CreateOptions{}, nil, fmt.Errorf("hostPath.path is required")
	}

	hostPath, err := filepath.Abs(spec.HostPath.Path)
	if err != nil {
		return dockervolume.CreateOptions{}, nil, fmt.Errorf("resolve host path: %w", err)
	}
	info, err := os.Stat(hostPath)
	if err != nil {
		return dockervolume.CreateOptions{}, nil, fmt.Errorf("host path %q is not accessible: %w", hostPath, err)
	}
	if !info.IsDir() {
		return dockervolume.CreateOptions{}, nil, fmt.Errorf("host path %q is not a directory", hostPath)
	}

	dockerVolumeName := fmt.Sprintf("%s_%s", sessionID, spec.Name)
	handle := HostPathHandle{
		HostPath:         hostPath,
		DockerVolumeName: dockerVolumeName,
	}

	return dockervolume.CreateOptions{
		Name:   dockerVolumeName,
		Driver: "local",
		DriverOpts: map[string]string{
			"type":   "none",
			"o":      "bind",
			"device": hostPath,
		},
		Labels: map[string]string{
			"dse.session": sessionID,
			"dse.kind":    string(spec.Kind),
			"dse.name":    spec.Name,
			"dse.volume":  dockerVolumeName,
		},
	}, handle, nil
}
