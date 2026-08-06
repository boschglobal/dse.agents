// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"fmt"
	"time"

	"github.com/boschglobal/dse.agents/mcp/pkg/volume"
)

func (s *Session) AddVolume(vol *volume.Volume) error {
	if vol == nil {
		return fmt.Errorf("volume is nil")
	}
	if vol.Name == "" {
		return fmt.Errorf("volume name is empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.volumes == nil {
		s.volumes = make(map[string]*volume.Volume)
	}
	if _, exists := s.volumes[vol.Name]; exists {
		return fmt.Errorf("volume %q already exists in session %q", vol.Name, s.ID)
	}
	s.volumes[vol.Name] = vol
	s.LastActivity = time.Now().UTC()
	return nil
}

func (s *Session) GetVolume(name string) (*volume.Volume, bool) {
	for _, vol := range s.volumes {
		if vol.Name == name {
			return vol, true
		}
	}
	return nil, false
}

func (s *Session) RemoveVolume(name string) error {
	delete(s.volumes, name)
	return nil
}
