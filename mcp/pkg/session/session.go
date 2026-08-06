// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"fmt"
	"sync"
	"time"

	"github.com/boschglobal/dse.agents/mcp/pkg/volume"
)

type SessionState int

const (
	StateIdle SessionState = iota
	StateRunningTask
)

type Session struct {
	ID           string
	CreatedAt    time.Time
	LastActivity time.Time
	state        SessionState
	mu           sync.RWMutex
	volumes      map[string]*volume.Volume
}

func NewSession(id string) *Session {
	return &Session{
		ID:        id,
		CreatedAt: time.Now(),
		volumes:   make(map[string]*volume.Volume),
	}
}

func (s *Session) State() SessionState {
	return s.state
}

func (s *Session) Acquire() (SessionState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == StateIdle {
		s.state = StateRunningTask
		s.LastActivity = time.Now()
		return s.state, nil
	} else {
		return s.state, fmt.Errorf("rejected: session in use")
	}
}

func (s *Session) Release() (SessionState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == StateRunningTask {
		s.state = StateIdle
		s.LastActivity = time.Now()
		return s.state, nil
	} else {
		return s.state, nil
	}
}

func (s *Session) IsCollectible() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state == StateIdle
}

func (s *Session) SetLastActivity(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastActivity = t
}

func (s *Session) IdleFor(now time.Time) time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return now.Sub(s.LastActivity)
}
