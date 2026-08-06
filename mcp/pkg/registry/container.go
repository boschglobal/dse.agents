// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package registry

type ContainerSpec struct {
	Image string
	Env   map[string]string

	Volume          map[string]string
	VolumeGenerator func(payload interface{}) map[string]string

	UseEntrypoint    bool
	CommandGenerator func(payload interface{}) []string

	// Prefer CommandGenerator and UseEntrypoint=true.
	CommandFormatter func(payload interface{}) string
}
