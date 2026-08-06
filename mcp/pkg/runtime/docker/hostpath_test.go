// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/boschglobal/dse.agents/mcp/pkg/volume"
)

func hostPathVolumeProviderTestCase() volumeProviderTestCase {
	return volumeProviderTestCase{
		Name: "hostpath",
		NewVolume: func(t *testing.T, ctx context.Context, rt *DockerRuntime, sessionID string, name string, mountPath string, readOnly bool) *volume.Volume {
			t.Helper()

			return newIntegrationVolumeWithSpec(t, ctx, rt, sessionID, volume.Spec{
				Name:      name,
				Kind:      volume.KindHostPath,
				MountPath: mountPath,
				ReadOnly:  readOnly,
				HostPath: &volume.HostPathSpec{
					Path: t.TempDir(),
				},
			})
		},
		AfterWriteAssert: func(t *testing.T, vol *volume.Volume, filePath string, want []byte) {
			t.Helper()

			handle, ok := vol.Handle.(HostPathHandle)
			if !ok {
				t.Fatalf("volume handle = %T, want HostPathHandle", vol.Handle)
			}

			got, err := os.ReadFile(filepath.Join(handle.HostPath, filePath))
			if err != nil {
				t.Fatalf("os.ReadFile(host %s) error = %v", filePath, err)
			}
			if string(got) != string(want) {
				t.Fatalf("host %s = %q, want %q", filePath, got, want)
			}
		},
		CreateVolumeWithDuplicateName: func(t *testing.T, ctx context.Context, rt *DockerRuntime, sessionID string, existing *volume.Volume) (*volume.Volume, error) {
			t.Helper()

			return rt.CreateVolume(ctx, sessionID, volume.Spec{
				Name:      existing.Name,
				Kind:      volume.KindHostPath,
				MountPath: "/different-workspace",
				HostPath: &volume.HostPathSpec{
					Path: t.TempDir(),
				},
			}, nil)
		},
		CreateVolumeWithDuplicateMountPath: func(t *testing.T, ctx context.Context, rt *DockerRuntime, sessionID string, existing *volume.Volume) (*volume.Volume, error) {
			t.Helper()

			return rt.CreateVolume(ctx, sessionID, volume.Spec{
				Name:      "different-workspace",
				Kind:      volume.KindHostPath,
				MountPath: existing.MountPath,
				HostPath: &volume.HostPathSpec{
					Path: t.TempDir(),
				},
			}, nil)
		},
	}
}

func (s *Suite) TestDockerRuntimeHostPathReadWriteAndList() {
	s.runVolumeProviderReadWriteAndList(hostPathVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeHostPathDeleteInvalidatesVolume() {
	s.runVolumeProviderDeleteInvalidatesVolume(hostPathVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeHostPathReadOnlyRejectsWrites() {
	s.runVolumeProviderReadOnlyRejectsWrites(hostPathVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeHostPathRejectsUnsafePaths() {
	s.runVolumeProviderRejectsUnsafePaths(hostPathVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeHostPathReadNonExistingFileFails() {
	s.runVolumeProviderReadNonExistingFileFails(hostPathVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeHostPathListNonExistingDirectoryReturnsEmptyList() {
	s.runVolumeProviderListNonExistingDirectoryReturnsEmptyList(hostPathVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeHostPathWriteVolumeFileRejectsInvalidPaths() {
	s.runVolumeProviderWriteVolumeFileRejectsInvalidPaths(hostPathVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeHostPathReadVolumeFileRejectsInvalidPaths() {
	s.runVolumeProviderReadVolumeFileRejectsInvalidPaths(hostPathVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeHostPathListVolumeFilesRejectsEscapingPaths() {
	s.runVolumeProviderListVolumeFilesRejectsEscapingPaths(hostPathVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeHostPathListVolumeFilesAllowsRootPaths() {
	s.runVolumeProviderListVolumeFilesAllowsRootPaths(hostPathVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeHostPathCreateVolumeRejectsDuplicateName() {
	s.runVolumeProviderCreateVolumeRejectsDuplicateName(hostPathVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeHostPathCreateVolumeRejectsDuplicateMountPath() {
	s.runVolumeProviderCreateVolumeRejectsDuplicateMountPath(hostPathVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeHostPathDeleteVolumeIgnoresNonExistingVolume() {
	s.runVolumeProviderDeleteVolumeIgnoresNonExistingVolume(hostPathVolumeProviderTestCase())
}
