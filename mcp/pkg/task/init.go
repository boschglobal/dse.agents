// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"reflect"

	"github.com/boschglobal/dse.agents/mcp/pkg/registry"
)

type ContainerTaskRequest interface {
	BuilderRequest |
		SimerRequest |
		MessageRequest |
		FileprintRequest
}

func init() {
	registry.Registry[reflect.TypeOf(MessageRequest{})] = NewMessageRequestSpec()
	registry.Registry[reflect.TypeOf(FileprintRequest{})] = NewFileprintRequestSpec()
	registry.Registry[reflect.TypeOf(SimerRequest{})] = NewSimerRequestSpec()
}
