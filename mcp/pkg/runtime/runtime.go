// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"fmt"

	"github.com/boschglobal/dse.agents/mcp/pkg/runtime/docker"
	"github.com/boschglobal/dse.agents/mcp/pkg/runtime/mock"
	"github.com/boschglobal/dse.agents/mcp/pkg/service"
)

func NewRuntime(driverType string) (service.Runtime, error) {
	switch driverType {
	case "docker":
		return docker.NewDockerRuntime()
	case "mock":
		return mock.NewMockRuntime(), nil
	default:
		return nil, fmt.Errorf("unknown runtime driver type %q", driverType)
	}
}
