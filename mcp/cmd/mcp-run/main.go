// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

type envelope map[string]any

type responseCollector struct {
	mu       sync.Mutex
	messages []envelope
}

func (c *responseCollector) add(msg envelope) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, msg)
}

func (c *responseCollector) all() []envelope {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]envelope, len(c.messages))
	copy(out, c.messages)
	return out
}

type progressCollector struct {
	mu     sync.Mutex
	stdout strings.Builder
	stderr strings.Builder
}

func (c *progressCollector) add(msg envelope) {
	method, _ := msg["method"].(string)
	if method != "notifications/progress" {
		return
	}

	params, _ := msg["params"].(map[string]any)
	meta, _ := params["meta"].(map[string]any)
	if meta == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if v, ok := meta["stdout"]; ok {
		if s, ok := v.(string); ok {
			c.stdout.WriteString(s)
		}
	}
	if v, ok := meta["stderr"]; ok {
		if s, ok := v.(string); ok {
			c.stderr.WriteString(s)
		}
	}
}

func (c *progressCollector) stdoutString() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stdout.String()
}

func (c *progressCollector) stderrString() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stderr.String()
}

func main() {
	var (
		waitFlag            bool
		pollInterval        time.Duration
		timeout             time.Duration
		normalizeTaskID     bool
		normalizeTimestamps bool
		mcpMode             string
		serverCmd           string
	)

	flag.BoolVar(&waitFlag, "wait", false, "wait for async tasks to reach terminal state and fetch tasks/result")
	flag.DurationVar(&pollInterval, "poll-interval", 500*time.Millisecond, "poll interval for -wait")
	flag.DurationVar(&timeout, "timeout", 10*time.Second, "timeout for reading responses / waiting for tasks")
	flag.BoolVar(&normalizeTaskID, "normalize-task-id", false, "normalize taskId/progressToken fields to <task-id>")
	flag.BoolVar(&normalizeTimestamps, "normalize-timestamps", false, "normalize RFC3339 timestamp fields to <time>")
	flag.StringVar(&mcpMode, "mcp", "http", "MCP Server with 'http' (default) or 'stdio'.")
	flag.StringVar(&serverCmd, "server", "", "MCP Server command.")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "usage: %s [flags] <session.json>\n", os.Args[0])
		os.Exit(2)
	}
	sessionPath := flag.Arg(0)

	requestLines, err := loadSession(sessionPath)
	if err != nil {
		failf("load session: %v", err)
	}

	var messages []envelope
	var containerStdout string
	var containerStderr string
	var procStderr []byte
	if mcpMode == "http" {
		messages, containerStdout, containerStderr, procStderr, err = runHTTPSession(
			serverCmd,
			requestLines,
			waitFlag,
			pollInterval,
			timeout,
			normalizeTaskID,
			normalizeTimestamps,
		)
	} else {
		messages, containerStdout, containerStderr, procStderr, err = runStdioSession(
			serverCmd,
			requestLines,
			waitFlag,
			pollInterval,
			timeout,
			normalizeTaskID,
			normalizeTimestamps,
		)
	}
	if err != nil {
		failWithStderr(procStderr, "%v", err)
	}

	for _, msg := range messages {
		b, err := json.Marshal(msg)
		if err != nil {
			failWithStderr(procStderr, "marshal output: %v", err)
		}
		fmt.Println(string(b))
	}

	if err := os.WriteFile("stdout.txt", []byte(containerStdout), 0644); err != nil {
		failWithStderr(procStderr, "write stdout.txt: %v", err)
	}
	if err := os.WriteFile("stderr.txt", []byte(containerStderr), 0644); err != nil {
		failWithStderr(procStderr, "write stderr.txt: %v", err)
	}

	if len(procStderr) > 0 {
		fmt.Fprintln(os.Stdout, "--- stderr ---")
		if !bytes.HasSuffix(procStderr, []byte("\n")) {
			_, _ = os.Stderr.Write(procStderr)
			_, _ = os.Stderr.Write([]byte("\n"))
		} else {
			_, _ = os.Stderr.Write(procStderr)
		}
	}

	fmt.Fprintln(os.Stdout, "--- container stdout ---")
	fmt.Fprint(os.Stdout, containerStdout)
	if !strings.HasSuffix(containerStdout, "\n") {
		fmt.Fprintln(os.Stdout)
	}

	fmt.Fprintln(os.Stderr, "--- container stderr ---")
	fmt.Fprint(os.Stderr, containerStderr)
	if !strings.HasSuffix(containerStderr, "\n") {
		fmt.Fprintln(os.Stderr)
	}
}

func loadSession(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines, scanner.Err()
}

func countExpectedResponses(lines []string) int {
	n := 0
	for _, line := range lines {
		var msg map[string]any
		if json.Unmarshal([]byte(line), &msg) != nil {
			continue
		}
		if _, hasMethod := msg["method"]; hasMethod {
			if _, hasID := msg["id"]; hasID {
				n++
			}
		}
	}
	return n
}

func maxRequestID(lines []string) int {
	maxID := 0
	for _, line := range lines {
		var msg map[string]any
		if json.Unmarshal([]byte(line), &msg) != nil {
			continue
		}
		v, ok := msg["id"]
		if !ok {
			continue
		}
		switch id := v.(type) {
		case float64:
			if int(id) > maxID {
				maxID = int(id)
			}
		}
	}
	return maxID
}

func sendJSON(w *bufio.Writer, msg map[string]any) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	if err := w.WriteByte('\n'); err != nil {
		return err
	}
	return w.Flush()
}

func readJSONLines(r io.Reader, out chan<- envelope) error {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var msg envelope
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			return fmt.Errorf("invalid JSON from server stdout: %w; line=%q", err, line)
		}
		out <- msg
	}
	return scanner.Err()
}

func waitForResponse(
	responses <-chan envelope,
	timeout time.Duration,
	collector *responseCollector,
	progress *progressCollector,
	normalizeTaskID bool,
	normalizeTimestamps bool,
) (envelope, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case msg, ok := <-responses:
			if !ok {
				return nil, fmt.Errorf("response stream closed")
			}
			if normalizeTaskID || normalizeTimestamps {
				normalizeMessage(msg, normalizeTaskID, normalizeTimestamps)
			}
			collector.add(msg)
			progress.add(msg)
			if _, hasID := msg["id"]; hasID {
				return msg, nil
			}
		case <-timer.C:
			return nil, fmt.Errorf("timeout after %s", timeout)
		}
	}
}

func extractTaskID(msg envelope) (string, bool) {
	result, ok := msg["result"].(map[string]any)
	if !ok {
		return "", false
	}
	task, ok := result["task"].(map[string]any)
	if !ok {
		return "", false
	}
	taskID, ok := task["taskId"].(string)
	return taskID, ok && taskID != ""
}

func extractTaskStatus(msg envelope) (string, bool) {
	result, ok := msg["result"].(map[string]any)
	if !ok {
		return "", false
	}
	if status, ok := result["status"].(string); ok {
		return status, true
	}
	if task, ok := result["task"].(map[string]any); ok {
		if status, ok := task["status"].(string); ok {
			return status, true
		}
	}
	return "", false
}

func isTerminalStatus(status string) bool {
	switch status {
	case "completed", "failed", "cancelled":
		return true
	default:
		return false
	}
}

func normalizeMessage(msg envelope, normalizeTaskID, normalizeTimestamps bool) {
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, val := range x {
				if normalizeTaskID && (k == "taskId" || k == "progressToken") {
					if _, ok := val.(string); ok {
						x[k] = "<task-id>"
						continue
					}
				}
				if normalizeTimestamps && (k == "createdAt" || k == "lastUpdatedAt") {
					if _, ok := val.(string); ok {
						x[k] = "<time>"
						continue
					}
				}
				walk(val)
			}
		case []any:
			for _, item := range x {
				walk(item)
			}
		}
	}
	walk(msg)
}

func failf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func failAndDrain(cmd *exec.Cmd, stderrBuf *bytes.Buffer, format string, args ...any) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}
	failWithStderr(stderrBuf.Bytes(), format, args...)
}

// func killAndWait(cmd *exec.Cmd) error {
// 	if cmd == nil || cmd.Process == nil {
// 		return nil
// 	}
// 	_ = cmd.Process.Kill()
// 	_, err := cmd.Process.Wait()
// 	return err
// }

func stopAndWait(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		// If process already exited, just wait for the real result.
		select {
		case err := <-done:
			return err
		case <-time.After(1 * time.Second):
			return err
		}
	}

	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		if killErr := cmd.Process.Kill(); killErr != nil {
			select {
			case err := <-done:
				return err
			case <-time.After(1 * time.Second):
				return killErr
			}
		}
		return <-done
	}
}

func failWithStderr(stderr []byte, format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	if len(stderr) > 0 {
		if !bytes.HasSuffix(stderr, []byte("\n")) {
			_, _ = os.Stderr.Write(stderr)
			_, _ = os.Stderr.Write([]byte("\n"))
		} else {
			_, _ = os.Stderr.Write(stderr)
		}
	}
	os.Exit(1)
}
