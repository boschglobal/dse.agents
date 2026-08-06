// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package registry

import "reflect"

var (
	Registry map[reflect.Type]ContainerSpec = map[reflect.Type]ContainerSpec{}
)
