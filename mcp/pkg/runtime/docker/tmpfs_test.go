// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"context"
	"testing"

	"github.com/boschglobal/dse.agents/mcp/pkg/volume"
)

func tmpfsVolumeProviderTestCase() volumeProviderTestCase {
	return volumeProviderTestCase{
		Name: "tmpfs",
		NewVolume: func(t *testing.T, ctx context.Context, rt *DockerRuntime, sessionID string, name string, mountPath string, readOnly bool) *volume.Volume {
			t.Helper()

			return newIntegrationVolumeWithSpec(t, ctx, rt, sessionID, volume.Spec{
				Name:      name,
				Kind:      volume.KindTmpFS,
				MountPath: mountPath,
				ReadOnly:  readOnly,
				TmpFS: &volume.TmpFSSpec{
					Size: "8m",
				},
			})
		},
		CreateVolumeWithDuplicateName: func(t *testing.T, ctx context.Context, rt *DockerRuntime, sessionID string, existing *volume.Volume) (*volume.Volume, error) {
			t.Helper()

			return rt.CreateVolume(ctx, sessionID, volume.Spec{
				Name:      existing.Name,
				Kind:      volume.KindTmpFS,
				MountPath: "/different-workspace",
				TmpFS: &volume.TmpFSSpec{
					Size: "8m",
				},
			}, nil)
		},
		CreateVolumeWithDuplicateMountPath: func(t *testing.T, ctx context.Context, rt *DockerRuntime, sessionID string, existing *volume.Volume) (*volume.Volume, error) {
			t.Helper()

			return rt.CreateVolume(ctx, sessionID, volume.Spec{
				Name:      "different-workspace",
				Kind:      volume.KindTmpFS,
				MountPath: existing.MountPath,
				TmpFS: &volume.TmpFSSpec{
					Size: "8m",
				},
			}, nil)
		},
	}
}

func (s *Suite) TestDockerRuntimeTmpfsReadWriteAndList() {
	s.runVolumeProviderReadWriteAndList(tmpfsVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeTmpfsDeleteInvalidatesVolume() {
	s.runVolumeProviderDeleteInvalidatesVolume(tmpfsVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeTmpfsReadOnlyRejectsWrites() {
	s.runVolumeProviderReadOnlyRejectsWrites(tmpfsVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeTmpfsRejectsUnsafePaths() {
	s.runVolumeProviderRejectsUnsafePaths(tmpfsVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeTmpfsReadNonExistingFileFails() {
	s.runVolumeProviderReadNonExistingFileFails(tmpfsVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeTmpfsListNonExistingDirectoryReturnsEmptyList() {
	s.runVolumeProviderListNonExistingDirectoryReturnsEmptyList(tmpfsVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeTmpfsWriteVolumeFileRejectsInvalidPaths() {
	s.runVolumeProviderWriteVolumeFileRejectsInvalidPaths(tmpfsVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeTmpfsReadVolumeFileRejectsInvalidPaths() {
	s.runVolumeProviderReadVolumeFileRejectsInvalidPaths(tmpfsVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeTmpfsListVolumeFilesRejectsEscapingPaths() {
	s.runVolumeProviderListVolumeFilesRejectsEscapingPaths(tmpfsVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeTmpfsListVolumeFilesAllowsRootPaths() {
	s.runVolumeProviderListVolumeFilesAllowsRootPaths(tmpfsVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeTmpfsCreateVolumeRejectsDuplicateName() {
	s.runVolumeProviderCreateVolumeRejectsDuplicateName(tmpfsVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeTmpfsCreateVolumeRejectsDuplicateMountPath() {
	s.runVolumeProviderCreateVolumeRejectsDuplicateMountPath(tmpfsVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeTmpfsDeleteVolumeIgnoresNonExistingVolume() {
	s.runVolumeProviderDeleteVolumeIgnoresNonExistingVolume(tmpfsVolumeProviderTestCase())
}
