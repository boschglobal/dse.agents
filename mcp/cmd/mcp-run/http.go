// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type httpRPCSession struct {
	client    *http.Client
	url       string
	sessionID string
}

func runHTTPSession(
	serverCmd string,
	requestLines []string,
	waitFlag bool,
	pollInterval time.Duration,
	timeout time.Duration,
	normalizeTaskID bool,
	normalizeTimestamps bool,
) ([]envelope, string, string, []byte, error) {
	var (
		cmd           *exec.Cmd
		stderrBuf     bytes.Buffer
		stderrWg      sync.WaitGroup
		startedServer bool
	)

	if serverCmd != "" {
		cmd = exec.Command(serverCmd, "-v", "-mcp=http")

		stderr, err := cmd.StderrPipe()
		if err != nil {
			return nil, "", "", nil, fmt.Errorf("stderr pipe: %w", err)
		}

		if err := cmd.Start(); err != nil {
			return nil, "", "", nil, fmt.Errorf("start server: %w", err)
		}
		startedServer = true

		stderrWg.Add(1)
		go func() {
			defer stderrWg.Done()
			_, _ = io.Copy(&stderrBuf, stderr)
		}()
	}

	var cleanupOnce sync.Once
	var stderrOut []byte

	cleanup := func() []byte {
		cleanupOnce.Do(func() {
			if startedServer {
				_ = stopAndWait(cmd)
				stderrWg.Wait()
			}
			stderrOut = append([]byte(nil), stderrBuf.Bytes()...)
		})
		return stderrOut
	}

	client := &http.Client{
		Timeout: timeout,
	}
	baseURL := "http://127.0.0.1:8080/mcp"
	session := &httpRPCSession{
		client: client,
		url:    baseURL,
	}

	if err := waitForHTTPServer(client, baseURL, 5*time.Second); err != nil {
		return nil, "", "", cleanup(), fmt.Errorf("wait for HTTP server: %w", err)
	}

	collector := &responseCollector{}
	progress := &progressCollector{}
	pendingTasks := []string{}
	nextID := maxRequestID(requestLines) + 1

	for _, line := range requestLines {
		var req envelope
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			return nil, "", "", cleanup(), fmt.Errorf("invalid request JSON: %w", err)
		}

		msgs, err := session.doHTTPRPC(req, normalizeTaskID, normalizeTimestamps)
		if err != nil {
			return nil, "", "", cleanup(), fmt.Errorf("send initial request: %w", err)
		}

		for _, msg := range msgs {
			collector.add(msg)
			progress.add(msg)
			if taskID, ok := extractTaskID(msg); ok {
				pendingTasks = append(pendingTasks, taskID)
			}
		}
	}

	if waitFlag {
		for _, taskID := range pendingTasks {
			terminal := false
			for !terminal {
				req := envelope{
					"jsonrpc": "2.0",
					"id":      nextID,
					"method":  "tasks/get",
					"params": map[string]any{
						"taskId": taskID,
					},
				}
				nextID++

				msgs, err := session.doHTTPRPC(req, normalizeTaskID, normalizeTimestamps)
				if err != nil {
					return nil, "", "", cleanup(), fmt.Errorf("send tasks/get: %w", err)
				}

				var resp envelope
				found := false
				for _, msg := range msgs {
					collector.add(msg)
					progress.add(msg)
					if _, ok := msg["id"]; ok {
						resp = msg
						found = true
					}
				}
				if !found {
					return nil, "", "", cleanup(), fmt.Errorf("tasks/get returned no response message with id")
				}

				status, ok := extractTaskStatus(resp)
				if ok && isTerminalStatus(status) {
					terminal = true
					break
				}

				time.Sleep(pollInterval)
			}

			req := envelope{
				"jsonrpc": "2.0",
				"id":      nextID,
				"method":  "tasks/result",
				"params": map[string]any{
					"taskId": taskID,
				},
			}
			nextID++

			msgs, err := session.doHTTPRPC(req, normalizeTaskID, normalizeTimestamps)
			if err != nil {
				return nil, "", "", cleanup(), fmt.Errorf("send tasks/result: %w", err)
			}
			for _, msg := range msgs {
				collector.add(msg)
				progress.add(msg)
			}
		}
	}

	stderr := cleanup()
	return collector.all(), progress.stdoutString(), progress.stderrString(), stderr, nil
}

func waitForHTTPServer(client *http.Client, url string, maxWait time.Duration) error {
	deadline := time.Now().Add(maxWait)

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout after %s", maxWait)
		}

		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader([]byte(`{}`)))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			return nil
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func (s *httpRPCSession) doHTTPRPC(
	reqMsg envelope,
	normalizeTaskID bool,
	normalizeTimestamps bool,
) ([]envelope, error) {
	b, err := json.Marshal(reqMsg)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, s.url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	if s.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", s.sessionID)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		s.sessionID = sid
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("http %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/event-stream") {
		msgs, err := readSSEMessages(resp.Body)
		if err != nil {
			return nil, err
		}
		for _, msg := range msgs {
			if normalizeTaskID || normalizeTimestamps {
				normalizeMessage(msg, normalizeTaskID, normalizeTimestamps)
			}
		}
		return msgs, nil
	}

	var msg envelope
	if err := json.NewDecoder(resp.Body).Decode(&msg); err != nil {
		return nil, fmt.Errorf("decode HTTP response: %w", err)
	}
	if normalizeTaskID || normalizeTimestamps {
		normalizeMessage(msg, normalizeTaskID, normalizeTimestamps)
	}
	return []envelope{msg}, nil
}

func readSSEMessages(r io.Reader) ([]envelope, error) {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var (
		msgs      []envelope
		dataLines []string
	)

	flush := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		payload := strings.Join(dataLines, "\n")
		dataLines = nil

		if payload == "" {
			return nil
		}

		var msg envelope
		if err := json.Unmarshal([]byte(payload), &msg); err != nil {
			return fmt.Errorf("invalid SSE JSON payload: %w; payload=%q", err, payload)
		}
		msgs = append(msgs, msg)
		return nil
	}

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			if err := flush(); err != nil {
				return nil, err
			}
			continue
		}

		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimPrefix(data, " ")
			dataLines = append(dataLines, data)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if err := flush(); err != nil {
		return nil, err
	}

	return msgs, nil
}
