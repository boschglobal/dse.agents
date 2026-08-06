// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package service

type LogType string

const (
	LogStdout LogType = "stdout"
	LogStderr LogType = "stderr"
	LogStatus LogType = "status"
)

type Log struct {
	Stream  LogType
	Message string
}

type Progress func(taskID string, log Log)

type TaskProgress func(taskID string, message string)

func (p TaskProgress) Report(taskID string, message string) {
	if p != nil {
		p(taskID, message)
	}
}
