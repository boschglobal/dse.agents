// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/boschglobal/dse.agents/mcp/pkg/volume"
)

type Manager struct {
	mu          sync.RWMutex
	sessions    map[string]*Session
	idleTimeout time.Duration
	volTeardown VolumeTeardownFunc
}

type VolumeTeardownFunc func(ctx context.Context, sessionID string, vol *volume.Volume) error

func NewManager(ctx context.Context, idleTimeout time.Duration, scanInterval time.Duration) *Manager {
	m := &Manager{
		sessions:    make(map[string]*Session),
		idleTimeout: idleTimeout,
	}
	go m.startVolumeReaper(ctx, scanInterval)
	slog.Debug("Session: new manager", "idleTimeout", idleTimeout)
	return m
}

func (m *Manager) SetVolumeTeardown(fn VolumeTeardownFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.volTeardown = fn
}

func (m *Manager) GetAllSessions() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return slices.Collect(maps.Values(m.sessions))
}

func (m *Manager) GetSession(id string) (*Session, bool) {
	if s, exists := m.sessions[id]; exists {
		return s, exists
	}
	return nil, false
}

func (m *Manager) CreateSession(id string) *Session {
	slog.Debug("Session: create", "id", id)
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, exists := m.sessions[id]; exists {
		return s
	}
	s := NewSession(id)
	m.sessions[id] = s
	return s
}

func (m *Manager) DestroySession(ctx context.Context, id string) error {
	slog.Debug("Session: destroy", "id", id)

	s := m.removeSession(id)
	if s == nil {
		return nil
	}
	return m.destroySession(ctx, s)
}

func (m *Manager) removeSession(id string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, exists := m.sessions[id]
	if !exists {
		return nil
	}
	delete(m.sessions, id)
	return s
}

func (m *Manager) destroySession(ctx context.Context, s *Session) error {
	var errs []error

	s.mu.Lock()
	volumes := make(map[string]*volume.Volume, len(s.volumes))
	for name, vol := range s.volumes {
		volumes[name] = vol
	}
	s.mu.Unlock()

	for name, vol := range volumes {
		slog.Debug("Session: volume teardown", "id", s.ID, "name", name)
		if m.volTeardown != nil {
			if err := m.volTeardown(ctx, s.ID, vol); err != nil {
				errs = append(errs, fmt.Errorf("failed tearing down volume %s: %w", name, err))
				continue
			}
		}

		s.mu.Lock()
		delete(s.volumes, name)
		s.mu.Unlock()
	}

	if len(errs) > 0 {
		return fmt.Errorf("session cleanup encountered errors: %v", errs)
	}
	return nil
}

func (m *Manager) startVolumeReaper(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.reapIdleSessions()
		case <-ctx.Done():
			m.shutdownAll(context.Background())
			return
		}
	}
}

func (m *Manager) reapIdleSessions() {
	m.mu.Lock()
	now := time.Now()
	expiredSessions := make([]*Session, 0)

	for id, s := range m.sessions {
		if !s.IsCollectible() {
			continue
		}
		if s.IdleFor(now) > m.idleTimeout {
			expiredSessions = append(expiredSessions, s)
			delete(m.sessions, id)
		}
	}
	m.mu.Unlock()

	for _, s := range expiredSessions {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		if err := m.destroySession(cleanupCtx, s); err != nil {
			slog.Error("Session: failed to destroy expired session", "id", s.ID, "err", err)
		}
		cancel()
	}
}

func (m *Manager) shutdownAll(ctx context.Context) {
	m.mu.Lock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	m.sessions = make(map[string]*Session)
	m.mu.Unlock()

	for _, s := range sessions {
		if err := m.destroySession(ctx, s); err != nil {
			slog.Error("Session: failed to destroy session during shutdown", "id", s.ID, "err", err)
		}
	}
}
