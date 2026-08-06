// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

func runStdioSession(
	serverCmd string,
	requestLines []string,
	waitFlag bool,
	pollInterval time.Duration,
	timeout time.Duration,
	normalizeTaskID bool,
	normalizeTimestamps bool,
) ([]envelope, string, string, []byte, error) {

	cmd := exec.Command(serverCmd, "-v", "-mcp=stdio")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		failf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		failf("stdout pipe: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		failf("stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		failf("start server: %v", err)
	}

	var stderrBuf bytes.Buffer
	var stderrWg sync.WaitGroup
	stderrWg.Add(1)
	go func() {
		defer stderrWg.Done()
		_, _ = io.Copy(&stderrBuf, stderr)
	}()

	collector := &responseCollector{}
	progress := &progressCollector{}
	responses := make(chan envelope, 128)
	readErrCh := make(chan error, 1)

	var stdoutWg sync.WaitGroup
	stdoutWg.Add(1)
	go func() {
		defer stdoutWg.Done()
		readErrCh <- readJSONLines(stdout, responses)
		close(responses)
	}()

	expectedInitialResponses := countExpectedResponses(requestLines)

	writer := bufio.NewWriter(stdin)
	for _, line := range requestLines {
		if _, err := writer.WriteString(line); err != nil {
			failAndDrain(cmd, &stderrBuf, "write request: %v", err)
		}
		if !strings.HasSuffix(line, "\n") {
			if err := writer.WriteByte('\n'); err != nil {
				failAndDrain(cmd, &stderrBuf, "write newline: %v", err)
			}
		}
	}
	if err := writer.Flush(); err != nil {
		failAndDrain(cmd, &stderrBuf, "flush requests: %v", err)
	}

	nextID := maxRequestID(requestLines) + 1
	pendingTasks := []string{}

	deadline := time.Now().Add(timeout)

	initialResponses := 0
	for initialResponses < expectedInitialResponses {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			failAndDrain(
				cmd,
				&stderrBuf,
				"timeout waiting for initial responses (%d/%d)",
				initialResponses,
				expectedInitialResponses,
			)
		}

		select {
		case msg, ok := <-responses:
			if !ok {
				failAndDrain(
					cmd,
					&stderrBuf,
					"server stdout closed before initial responses completed (%d/%d)",
					initialResponses,
					expectedInitialResponses,
				)
			}
			if normalizeTaskID || normalizeTimestamps {
				normalizeMessage(msg, normalizeTaskID, normalizeTimestamps)
			}
			collector.add(msg)
			progress.add(msg)
			if _, ok := msg["id"]; ok {
				initialResponses++
			}
			if taskID, ok := extractTaskID(msg); ok {
				pendingTasks = append(pendingTasks, taskID)
			}
		case <-time.After(remaining):
			failAndDrain(
				cmd,
				&stderrBuf,
				"timeout waiting for initial responses (%d/%d)",
				initialResponses,
				expectedInitialResponses,
			)
		}
	}

	if waitFlag {
		for _, taskID := range pendingTasks {
			terminal := false
			for !terminal {
				req := map[string]any{
					"jsonrpc": "2.0",
					"id":      nextID,
					"method":  "tasks/get",
					"params": map[string]any{
						"taskId": taskID,
					},
				}
				nextID++

				if err := sendJSON(writer, req); err != nil {
					failAndDrain(cmd, &stderrBuf, "send tasks/get: %v", err)
				}

				msg, err := waitForResponse(
					responses,
					timeout,
					collector,
					progress,
					normalizeTaskID,
					normalizeTimestamps,
				)
				if err != nil {
					failAndDrain(cmd, &stderrBuf, "wait for tasks/get response: %v", err)
				}

				status, ok := extractTaskStatus(msg)
				if ok && isTerminalStatus(status) {
					terminal = true
					break
				}

				time.Sleep(pollInterval)
			}

			req := map[string]any{
				"jsonrpc": "2.0",
				"id":      nextID,
				"method":  "tasks/result",
				"params": map[string]any{
					"taskId": taskID,
				},
			}
			nextID++

			if err := sendJSON(writer, req); err != nil {
				failAndDrain(cmd, &stderrBuf, "send tasks/result: %v", err)
			}

			if _, err := waitForResponse(
				responses,
				timeout,
				collector,
				progress,
				normalizeTaskID,
				normalizeTimestamps,
			); err != nil {
				failAndDrain(cmd, &stderrBuf, "wait for tasks/result response: %v", err)
			}
		}
		time.Sleep(250 * time.Millisecond)
	}

	_ = stdin.Close()

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	stdoutWg.Wait()
	stderrWg.Wait()
	waitErr := <-waitCh

	select {
	case err := <-readErrCh:
		if err != nil && err != io.EOF {
			return nil, "", "", stderrBuf.Bytes(), fmt.Errorf("read stdout: %w", err)
		}
	default:
	}

	if waitErr != nil {
		return nil, "", "", stderrBuf.Bytes(), fmt.Errorf("server exit: %w", waitErr)
	}

	return collector.all(), progress.stdoutString(), progress.stderrString(), stderrBuf.Bytes(), nil
}
