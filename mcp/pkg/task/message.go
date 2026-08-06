// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"github.com/boschglobal/dse.agents/mcp/pkg/registry"
)

type MessageRequest struct {
	Message string `json:"message" doc:"The message will be printed to stdout." validate:"required"`
}

func NewMessageRequestSpec() registry.ContainerSpec {
	return registry.ContainerSpec{
		Image:         "dse-message:test",
		UseEntrypoint: true,
		CommandGenerator: func(p interface{}) []string {
			req := p.(MessageRequest)
			return []string{"--msg", req.Message}
		},
	}
}
