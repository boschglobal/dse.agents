// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"context"
	"testing"

	"github.com/boschglobal/dse.agents/mcp/pkg/volume"
)

func ephemeralVolumeProviderTestCase() volumeProviderTestCase {
	return volumeProviderTestCase{
		Name: "ephemeral",
		NewVolume: func(t *testing.T, ctx context.Context, rt *DockerRuntime, sessionID string, name string, mountPath string, readOnly bool) *volume.Volume {
			t.Helper()

			return newIntegrationVolumeWithSpec(t, ctx, rt, sessionID, volume.Spec{
				Name:      name,
				Kind:      volume.KindEphemeral,
				MountPath: mountPath,
				ReadOnly:  readOnly,
			})
		},
		CreateVolumeWithDuplicateName: func(t *testing.T, ctx context.Context, rt *DockerRuntime, sessionID string, existing *volume.Volume) (*volume.Volume, error) {
			t.Helper()

			return rt.CreateVolume(ctx, sessionID, volume.Spec{
				Name:      existing.Name,
				Kind:      volume.KindEphemeral,
				MountPath: "/different-workspace",
			}, nil)
		},
		CreateVolumeWithDuplicateMountPath: func(t *testing.T, ctx context.Context, rt *DockerRuntime, sessionID string, existing *volume.Volume) (*volume.Volume, error) {
			t.Helper()

			return rt.CreateVolume(ctx, sessionID, volume.Spec{
				Name:      "different-workspace",
				Kind:      volume.KindEphemeral,
				MountPath: existing.MountPath,
			}, nil)
		},
	}
}

func (s *Suite) TestDockerRuntimeEphemeralReadWriteAndList() {
	s.runVolumeProviderReadWriteAndList(ephemeralVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeEphemeralDeleteInvalidatesVolume() {
	s.runVolumeProviderDeleteInvalidatesVolume(ephemeralVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeEphemeralReadOnlyRejectsWrites() {
	s.runVolumeProviderReadOnlyRejectsWrites(ephemeralVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeEphemeralRejectsUnsafePaths() {
	s.runVolumeProviderRejectsUnsafePaths(ephemeralVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeEphemeralReadNonExistingFileFails() {
	s.runVolumeProviderReadNonExistingFileFails(ephemeralVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeEphemeralListNonExistingDirectoryReturnsEmptyList() {
	s.runVolumeProviderListNonExistingDirectoryReturnsEmptyList(ephemeralVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeEphemeralWriteVolumeFileRejectsInvalidPaths() {
	s.runVolumeProviderWriteVolumeFileRejectsInvalidPaths(ephemeralVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeEphemeralReadVolumeFileRejectsInvalidPaths() {
	s.runVolumeProviderReadVolumeFileRejectsInvalidPaths(ephemeralVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeEphemeralListVolumeFilesRejectsEscapingPaths() {
	s.runVolumeProviderListVolumeFilesRejectsEscapingPaths(ephemeralVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeEphemeralListVolumeFilesAllowsRootPaths() {
	s.runVolumeProviderListVolumeFilesAllowsRootPaths(ephemeralVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeEphemeralCreateVolumeRejectsDuplicateName() {
	s.runVolumeProviderCreateVolumeRejectsDuplicateName(ephemeralVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeEphemeralCreateVolumeRejectsDuplicateMountPath() {
	s.runVolumeProviderCreateVolumeRejectsDuplicateMountPath(ephemeralVolumeProviderTestCase())
}

func (s *Suite) TestDockerRuntimeEphemeralDeleteVolumeIgnoresNonExistingVolume() {
	s.runVolumeProviderDeleteVolumeIgnoresNonExistingVolume(ephemeralVolumeProviderTestCase())
}
