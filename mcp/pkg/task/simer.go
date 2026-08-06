// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"strconv"
	"strings"

	"github.com/boschglobal/dse.agents/mcp/pkg/registry"
)

type SimerRequest struct {
	SimPath string            `json:"simpath"           doc:"Host relative path of the simulation which will be mounted into the Simer container. A simulation folder typically contains a file data/simulation.yaml. If the current folder does not contain that file, then look for a folder sim or out/sim which might contain the simulation." validate:"required"`
	EndTime float64           `json:"endtime"           doc:"Simulation duration boundary in seconds (e.g., 0.1). Maps to -endtime. If an endtime is not known then assume the simulation is being tested and choose a value of 1.0"                                                                                                               validate:"required"`
	Stack   []string          `json:"stack,omitempty"   doc:"List of named simulation stacks to run. If a value is not known then all stacks will be run."`
	Env     map[string]string `json:"env,omitempty"     doc:"Environment variables to be set in the simulation runtime environment. Use the Inspect Request to learn the available settings. Other values may also be valid but generally don't guess values."`
	Logger  int               `json:"logger,omitempty"  doc:"Log level sensitivity of the Simer tool. Select an integer value in the range; 1 (debug), 2 (info), 3 (warnings) or 4 (errors only). Default is 3."`
	Timeout int               `json:"timeout,omitempty" doc:"Maximum command execution timeout in seconds before force-aborting (default: 60)."`
}

func NewSimerRequestSpec() registry.ContainerSpec {
	return registry.ContainerSpec{
		Image:  "ghcr.io/boschglobal/dse-simer:latest",
		Env:    map[string]string{},
		Volume: map[string]string{},
		VolumeGenerator: func(p interface{}) map[string]string {
			req := p.(SimerRequest)
			return map[string]string{req.SimPath: "/sim"}
		},
		UseEntrypoint: true,
		CommandGenerator: func(p interface{}) []string {
			req := p.(SimerRequest)
			items := []string{}

			// Endtime
			if req.EndTime == 0 {
				req.EndTime = 1.0
			}
			items = append(items, "-endtime", strconv.FormatFloat(req.EndTime, 'f', -1, 64))
			// Stacks
			if (len(req.Stack)) > 0 {
				items = append(items, "-stack", strings.Join(req.Stack, ";"))
			}
			// Logger
			if req.Logger != 0 {
				items = append(items, "-logger", strconv.FormatInt(int64(req.Logger), 10))
			}
			// Timeout
			if req.Logger != 0 {
				items = append(items, "-timeout", strconv.FormatInt(int64(req.Timeout), 10))
			}
			// Envars
			for k, v := range req.Env {
				items = append(items, "-env", k+"="+v)
			}

			return items
		},
	}
}
