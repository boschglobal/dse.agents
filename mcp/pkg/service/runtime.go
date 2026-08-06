// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/boschglobal/dse.agents/mcp/pkg/registry"
	"github.com/boschglobal/dse.agents/mcp/pkg/volume"
)

type Runtime interface {
	Run(
		ctx context.Context,
		sessionID string,
		task *ContainerTask,
		spec registry.ContainerSpec,
		request interface{},
		onProgress Progress,
	) (ContainerResult, error)

	CreateVolume(
		ctx context.Context,
		sessionID string,
		spec volume.Spec,
		onProgress Progress,
	) (*volume.Volume, error)

	DeleteVolume(
		ctx context.Context,
		sessionID string,
		volume *volume.Volume,
		onProgress Progress,
	) error

	ListVolumeFiles(
		ctx context.Context,
		sessionID string,
		volume *volume.Volume,
		path string,
	) ([]volume.FileInfo, error)

	ReadVolumeFile(
		ctx context.Context,
		sessionID string,
		volume *volume.Volume,
		path string,
	) ([]byte, error)

	WriteVolumeFile(
		ctx context.Context,
		sessionID string,
		volume *volume.Volume,
		path string,
		data []byte,
	) error
}
