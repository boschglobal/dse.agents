// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package task

// Env map[string]string `json:"env,omitempty" doc:"Dynamic runtime environment modifiers mapped to model variables. Read the simulation configurations, Signal Map files, or model descriptions inside the workspace directory first to discover valid variable keys."`

// Env map[string]string `json:"env,omitempty" doc:"Dynamic runtime environment modifiers mapped to model variables. Useful platform variables include: 'DSE_LOG_LEVEL' (overrides internal engine logs), 'SIMBUS_LOG' (tracks bus signals), and model-specific signals in the format 'MODEL_NAME:VARIABLE_NAME=VALUE'."`

// // Pattern 3, a dedicated discovery tool

// // The LLM runs this tool FIRST when it wants to see what it can change
// type InspectSimRequest struct {
// 	SimPath string `json:"simpath" doc:"Path to the simulation workspace folder." validate:"required"`
// }

// type InspectSimResponse struct {
// 	AvailableStacks []string          `json:"available_stacks" doc:"Semicolon-delimited stack names found in the configuration."`
// 	ValidEnvVars    map[string]string `json:"valid_env_vars" doc:"Discovered model variables and their expected types or defaults."`
// }
