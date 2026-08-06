// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package mock

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/boschglobal/dse.agents/mcp/pkg/volume"
)

var errInvalidMemFSHandle = errors.New("invalid memfs handle")

type MemFSProvider struct {
	mu      sync.Mutex
	volumes map[string]*volume.Volume
}

type MemFSStore struct {
	mu    sync.RWMutex
	files map[string]*MemFSFile
}

type MemFSFile struct {
	Data     []byte
	Mode     string
	Modified time.Time
}

func NewMemFSProvider() *MemFSProvider {
	return &MemFSProvider{
		volumes: make(map[string]*volume.Volume),
	}
}

func memFSStore(vol *volume.Volume) (*MemFSStore, error) {
	if vol == nil {
		return nil, fmt.Errorf("volume is nil")
	}
	if vol.Kind != volume.KindMemFS {
		return nil, fmt.Errorf("volume %q is not memfs", vol.Name)
	}
	handle, ok := vol.Handle.(*MemFSStore)
	if !ok || handle == nil {
		return nil, fmt.Errorf("volume %q has invalid memfs handle: %w", vol.Name, errInvalidMemFSHandle)
	}
	return handle, nil
}

func (p *MemFSProvider) ListVolumes(ctx context.Context, sessionID string) ([]volume.Volume, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	vols := make([]volume.Volume, 0, len(p.volumes))
	for _, vol := range p.volumes {
		if vol == nil {
			continue
		}
		vols = append(vols, *vol)
	}
	return vols, nil
}

func (p *MemFSProvider) CreateVolume(ctx context.Context, sessionID string, spec volume.Spec) (*volume.Volume, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.volumes[spec.Name]; ok {
		return nil, fmt.Errorf("volume %q already exists", spec.Name)
	}
	for _, existing := range p.volumes {
		if existing.MountPath == spec.MountPath {
			return nil, fmt.Errorf("volume mount path %q already exists", spec.MountPath)
		}
	}

	store := &MemFSStore{
		files: make(map[string]*MemFSFile),
	}
	vol := &volume.Volume{
		SessionID: sessionID,
		Name:      spec.Name,
		Kind:      spec.Kind,
		MountPath: spec.MountPath,
		ReadOnly:  spec.ReadOnly,
		Handle:    store,
	}
	p.volumes[spec.Name] = vol
	return vol, nil
}

func (p *MemFSProvider) DeleteVolume(ctx context.Context, sessionID string, vol *volume.Volume) error {
	store, err := memFSStore(vol)
	if err != nil {
		if errors.Is(err, errInvalidMemFSHandle) {
			return nil
		}
		return err
	}

	store.mu.Lock()
	for k := range store.files {
		delete(store.files, k) // Force immediate GC.
	}
	store.files = nil
	store.mu.Unlock()

	p.mu.Lock()
	delete(p.volumes, vol.Name)
	p.mu.Unlock()

	vol.Handle = nil
	return nil
}

func (p *MemFSProvider) WriteVolumeFile(
	ctx context.Context,
	sessionID string,
	vol *volume.Volume,
	filePath string,
	data []byte,
) error {
	if vol.ReadOnly {
		return fmt.Errorf("volume %q is read-only", vol.Name)
	}

	store, err := memFSStore(vol)
	if err != nil {
		return err
	}

	clean, err := volume.CleanFilePath(filePath)
	if err != nil {
		return err
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	store.files[clean] = &MemFSFile{
		Data:     append([]byte(nil), data...),
		Mode:     "0644",
		Modified: time.Now().UTC(),
	}

	return nil
}

func (p *MemFSProvider) ReadVolumeFile(
	ctx context.Context,
	sessionID string,
	vol *volume.Volume,
	filePath string,
) ([]byte, error) {
	store, err := memFSStore(vol)
	if err != nil {
		return nil, err
	}

	clean, err := volume.CleanFilePath(filePath)
	if err != nil {
		return nil, err
	}

	store.mu.RLock()
	defer store.mu.RUnlock()

	file, ok := store.files[clean]
	if !ok {
		return nil, fmt.Errorf("file %q not found in volume %q", clean, vol.Name)
	}

	return append([]byte(nil), file.Data...), nil
}

func (p *MemFSProvider) ListVolumeFiles(
	ctx context.Context,
	sessionID string,
	vol *volume.Volume,
	dirPath string,
) ([]volume.FileInfo, error) {
	store, err := memFSStore(vol)
	if err != nil {
		return nil, err
	}

	clean, err := volume.CleanDirPath(dirPath)
	if err != nil {
		return nil, err
	}
	if clean == "." {
		clean = ""
	}

	prefix := clean
	if prefix != "" {
		prefix += "/"
	}

	store.mu.RLock()
	defer store.mu.RUnlock()

	seenDirs := map[string]bool{}
	var out []volume.FileInfo

	for filePath, file := range store.files {
		if !strings.HasPrefix(filePath, prefix) {
			continue
		}

		rest := strings.TrimPrefix(filePath, prefix)
		if rest == "" {
			continue
		}

		parts := strings.SplitN(rest, "/", 2)
		name := parts[0]
		childPath := path.Join(clean, name)

		if len(parts) > 1 {
			if seenDirs[childPath] {
				continue
			}
			seenDirs[childPath] = true

			out = append(out, volume.FileInfo{
				Path:  childPath,
				Name:  name,
				IsDir: true,
			})
			continue
		}

		out = append(out, volume.FileInfo{
			Path:     childPath,
			Name:     name,
			IsDir:    false,
			Size:     int64(len(file.Data)),
			Mode:     file.Mode,
			Modified: file.Modified.Format(time.RFC3339),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return out[i].Name < out[j].Name
	})

	return out, nil
}

func (p *MemFSProvider) VolumeExists(name string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	_, ok := p.volumes[name]
	return ok
}
