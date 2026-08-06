// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"github.com/boschglobal/dse.agents/mcp/pkg/registry"
)

type FileprintRequest struct {
	Path string `json:"" doc:"The content of the file referenced by path will be printed to stdout" validate:"required"`
}

func NewFileprintRequestSpec() registry.ContainerSpec {
	return registry.ContainerSpec{
		Image:         "dse-fileprint:test",
		UseEntrypoint: true,
		CommandGenerator: func(p interface{}) []string {
			req := p.(FileprintRequest)
			return []string{"--path", req.Path}
		},
	}
}
