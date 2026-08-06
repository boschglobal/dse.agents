// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package task

type BuilderRequest struct {
	ProjPath string            `json:"projpath" validate:"required"`
	Env      map[string]string `json:"env"      validate:"optional"` // TODO tokens and secrets ?
}

// // Builder
// cts.registry[reflect.TypeOf(task.BuilderRequest{})] = ContainerSpec{
// 	Image: "message:latest",
// 	CommandFormatter: func(p interface{}) string {
// 		//req := p.(task.BuilderRequest)
// 		return fmt.Sprintf("--msg=%q", "builder")
// 	},
// }
