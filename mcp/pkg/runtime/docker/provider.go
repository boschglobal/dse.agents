// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	_ "embed"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	dockervolume "github.com/docker/docker/api/types/volume"

	"github.com/boschglobal/dse.agents/mcp/pkg/volume"
)

type VolumeOptionsFunc func(ctx context.Context, sessionID string, spec volume.Spec) (dockervolume.CreateOptions, DockerVolumeHandle, error)
type VolumePostCreateFunc func(ctx context.Context, p *Provider, vol *volume.Volume) error
type VolumePreDeleteFunc func(ctx context.Context, p *Provider, vol *volume.Volume) error

type DockerVolumeHandle interface {
	DockerVolume() string
}

type ProviderConfig struct {
	Kind          volume.Kind
	LogName       string
	VolumeOptions VolumeOptionsFunc

	PostCreate VolumePostCreateFunc
	PreDelete  VolumePreDeleteFunc
}

type Provider struct {
	mu      sync.Mutex
	volumes map[string]*volume.Volume

	helperImageMu sync.Mutex

	runtime *DockerRuntime
	config  ProviderConfig
}

func dockerVolumeNameFromVolume(vol *volume.Volume) (string, error) {
	if vol == nil {
		return "", fmt.Errorf("volume is nil")
	}

	handle, ok := vol.Handle.(DockerVolumeHandle)
	if !ok {
		return "", fmt.Errorf("volume %q has unexpected handle type %T", vol.Name, vol.Handle)
	}

	dockerVolumeName := handle.DockerVolume()
	if dockerVolumeName == "" {
		return "", fmt.Errorf("docker volume name is empty for volume %q", vol.Name)
	}

	return dockerVolumeName, nil
}

func NewProvider(runtime *DockerRuntime, config ProviderConfig) *Provider {
	return &Provider{
		volumes: make(map[string]*volume.Volume),
		runtime: runtime,
		config:  config,
	}
}

func (p *Provider) ListVolumes(ctx context.Context, sessionID string) ([]volume.Volume, error) {
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

func (p *Provider) CreateVolume(ctx context.Context, sessionID string, spec volume.Spec) (*volume.Volume, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.config.VolumeOptions == nil {
		return nil, fmt.Errorf("docker volume provider %q has no volume options function", p.config.LogName)
	}

	if _, ok := p.volumes[spec.Name]; ok {
		return nil, fmt.Errorf("volume %q already exists", spec.Name)
	}
	for _, existing := range p.volumes {
		if existing.MountPath == spec.MountPath {
			return nil, fmt.Errorf("volume mount path %q already exists", spec.MountPath)
		}
	}

	if spec.Name == "" {
		return nil, fmt.Errorf("volume name is required")
	}
	if spec.MountPath == "" {
		return nil, fmt.Errorf("volume mountPath is required")
	}

	createOptions, handle, err := p.config.VolumeOptions(ctx, sessionID, spec)
	if err != nil {
		return nil, err
	}
	dockerVolumeName := handle.DockerVolume()
	if dockerVolumeName == "" {
		return nil, fmt.Errorf("docker volume name is empty")
	}

	slog.Debug("Create docker volume",
		"Provider", p.config.LogName,
		"DockerVolumeName", dockerVolumeName,
		"MountPath", spec.MountPath,
	)

	_, err = p.runtime.client.VolumeCreate(ctx, createOptions)
	if err != nil {
		return nil, fmt.Errorf("create docker volume %q for provider %q: %w",
			dockerVolumeName, p.config.LogName, err)
	}

	vol := &volume.Volume{
		SessionID: sessionID,
		Name:      spec.Name,
		Kind:      spec.Kind,
		MountPath: spec.MountPath,
		ReadOnly:  spec.ReadOnly,
		Handle:    handle,
	}
	if p.config.PostCreate != nil {
		if err := p.config.PostCreate(ctx, p, vol); err != nil {
			_ = p.runtime.client.VolumeRemove(ctx, dockerVolumeName, true)
			return nil, err
		}
	}

	p.volumes[spec.Name] = vol
	return vol, nil
}

func (p *Provider) DeleteVolume(ctx context.Context, sessionID string, vol *volume.Volume) error {
	if vol == nil {
		return fmt.Errorf("volume is nil")
	}
	if vol.Handle == nil {
		p.mu.Lock()
		delete(p.volumes, vol.Name)
		p.mu.Unlock()
		return nil
	}

	dockerVolumeName, err := dockerVolumeNameFromVolume(vol)
	if err != nil {
		return err
	}

	slog.Debug("Delete docker volume",
		"Provider", p.config.LogName,
		"DockerVolumeName", dockerVolumeName,
	)
	if p.config.PreDelete != nil {
		if err := p.config.PreDelete(ctx, p, vol); err != nil {
			return err
		}
	}
	if err := p.runtime.client.VolumeRemove(ctx, dockerVolumeName, true); err != nil {
		if !cerrdefs.IsNotFound(err) {
			return fmt.Errorf("delete docker volume %q: %w", dockerVolumeName, err)
		}
	}

	p.mu.Lock()
	delete(p.volumes, vol.Name)
	p.mu.Unlock()

	vol.Handle = nil
	return nil
}

func (p *Provider) WriteVolumeFile(
	ctx context.Context,
	sessionID string,
	vol *volume.Volume,
	filePath string,
	data []byte,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if vol.ReadOnly {
		return fmt.Errorf("volume %q is read-only", vol.Name)
	}

	dockerVolumeName, err := dockerVolumeNameFromVolume(vol)
	if err != nil {
		return err
	}
	relPath, err := cleanVolumeRelPath(filePath)
	if err != nil {
		return err
	}
	if relPath == "" {
		return fmt.Errorf("volume file path is empty")
	}
	archive, err := tarArchiveForFile(relPath, data)
	if err != nil {
		return err
	}

	slog.Debug("WriteVolumeFile", "dockerVolumeName", dockerVolumeName, "filePath", filePath, "relPath", relPath)
	return p.withVolumeHelperContainer(ctx, dockerVolumeName, false, func(containerID string) error {
		if err := p.runtime.client.CopyToContainer(
			ctx,
			containerID,
			helperMountPath,
			archive,
			container.CopyToContainerOptions{
				AllowOverwriteDirWithFile: false,
				CopyUIDGID:                false,
			},
		); err != nil {
			return fmt.Errorf("copy file %q to docker volume %q via helper container %q: %w",
				relPath, dockerVolumeName, containerID, err)
		}

		return nil
	})
}

func (p *Provider) ReadVolumeFile(
	ctx context.Context,
	sessionID string,
	vol *volume.Volume,
	filePath string,
) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	dockerVolumeName, err := dockerVolumeNameFromVolume(vol)
	if err != nil {
		return nil, err
	}
	relPath, err := cleanVolumeRelPath(filePath)
	if err != nil {
		return nil, err
	}
	if relPath == "" {
		return nil, fmt.Errorf("volume file path is empty")
	}
	containerPath := path.Join(helperMountPath, relPath)
	var data []byte

	slog.Debug("ReadVolumeFile", "dockerVolumeName", dockerVolumeName, "relPath", relPath)
	err = p.withVolumeHelperContainer(ctx, dockerVolumeName, true, func(containerID string) error {
		reader, _, err := p.runtime.client.CopyFromContainer(ctx, containerID, containerPath)
		if err != nil {
			return fmt.Errorf("copy file %q from docker volume %q via helper container %q: %w",
				relPath, dockerVolumeName, containerID, err)
		}
		defer reader.Close()

		tr := tar.NewReader(reader)
		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("read tar archive for %q: %w", relPath, err)
			}
			if header.FileInfo().IsDir() {
				return fmt.Errorf("volume path %q is a directory, not a file", relPath)
			}

			data, err = io.ReadAll(tr)
			if err != nil {
				return fmt.Errorf("read file %q from tar archive: %w", relPath, err)
			} else {
				return nil
			}
		}

		return fmt.Errorf("file %q not found in docker volume %q", relPath, dockerVolumeName)
	})
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (p *Provider) ListVolumeFiles(
	ctx context.Context,
	sessionID string,
	vol *volume.Volume,
	dirPath string,
) ([]volume.FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	dockerVolumeName, err := dockerVolumeNameFromVolume(vol)
	if err != nil {
		return nil, err
	}
	relDirPath, err := cleanVolumeRelPath(dirPath)
	if err != nil {
		return nil, err
	}
	containerPath := helperMountPath
	if relDirPath != "" {
		containerPath = path.Join(helperMountPath, relDirPath)
	}
	slog.Debug(
		"ListVolumeFiles",
		"dockerVolumeName",
		dockerVolumeName,
		"relDirPath",
		relDirPath,
		"containerPath",
		containerPath,
	)

	copyPath := path.Clean(containerPath) + "/."
	var files []volume.FileInfo
	err = p.withVolumeHelperContainer(ctx, dockerVolumeName, true, func(containerID string) error {
		reader, _, err := p.runtime.client.CopyFromContainer(ctx, containerID, copyPath)
		if err != nil {
			if cerrdefs.IsNotFound(err) {
				files = []volume.FileInfo{}
				return nil
			}
			return fmt.Errorf("copy directory %q from docker volume %q via helper container %q: %w",
				relDirPath, dockerVolumeName, containerID, err)
		}
		defer reader.Close()

		tr := tar.NewReader(reader)
		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("read tar archive for directory %q: %w", relDirPath, err)
			}

			name := path.Clean(header.Name)
			if name == "." || name == "/" {
				continue
			}
			if strings.Contains(name, "/") {
				continue
			}
			filePath := name
			if relDirPath != "" {
				filePath = path.Join(relDirPath, name)
			}
			info := header.FileInfo()
			fi := volume.FileInfo{
				Path:     filePath,
				Name:     name,
				IsDir:    info.IsDir(),
				Size:     header.Size,
				Mode:     info.Mode().String(),
				Modified: header.ModTime.Format(time.RFC3339),
			}
			slog.Debug("ListVolumeFile", slog.Any("file", fi))
			files = append(files, fi)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}

func (p *Provider) withVolumeHelperContainer(
	ctx context.Context,
	dockerVolumeName string,
	readOnly bool,
	fn func(containerID string) error,
) error {
	containerID, err := p.createVolumeHelperContainer(ctx, dockerVolumeName, readOnly, []string{"true"})
	if err != nil {
		return err
	}

	defer func() {
		if err := p.runtime.client.ContainerRemove(ctx, containerID, container.RemoveOptions{
			Force:         true,
			RemoveVolumes: false,
		}); err != nil {
			slog.Warn("Failed to remove volume helper container", "ContainerID", containerID, "Error", err)
		}
	}()

	return fn(containerID)
}

func (p *Provider) createVolumeHelperContainer(
	ctx context.Context,
	dockerVolumeName string,
	readOnly bool,
	cmd []string,
) (string, error) {
	if dockerVolumeName == "" {
		return "", fmt.Errorf("docker volume name is empty")
	}
	if err := p.ensureHelperImage(ctx); err != nil {
		return "", err
	}

	resp, err := p.runtime.client.ContainerCreate(
		ctx,
		&container.Config{
			Image: helperImage,
			User:  fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
			Cmd:   cmd,
		},
		&container.HostConfig{
			AutoRemove: false,
			Mounts: []mount.Mount{
				{
					Type:     mount.TypeVolume,
					Source:   dockerVolumeName,
					Target:   helperMountPath,
					ReadOnly: readOnly,
				},
			},
		},
		nil,
		nil,
		"",
	)
	if err != nil {
		return "", fmt.Errorf("create helper container for docker volume %q: %w", dockerVolumeName, err)
	}

	return resp.ID, nil
}

const helperMountPath = "/volume"
const helperImage = "dse-helper:test"

//go:embed dockerfiles/volume-helper.Dockerfile
var helperImageDockerfile string

func (p *Provider) ensureHelperImage(ctx context.Context) error {
	p.helperImageMu.Lock()
	defer p.helperImageMu.Unlock()

	if helperImage == "" {
		return fmt.Errorf("helper image is empty")
	}

	_, err := p.runtime.client.ImageInspect(ctx, helperImage)
	if err == nil {
		return nil
	}
	if !cerrdefs.IsNotFound(err) {
		return fmt.Errorf("inspect helper image %q: %w", helperImage, err)
	}
	slog.Info("Building Docker volume helper image", "Image", helperImage)

	buildContext, err := dockerfileBuildContext(helperImageDockerfile)
	if err != nil {
		return err
	}
	defer buildContext.Close()

	resp, err := p.runtime.client.ImageBuild(ctx, buildContext, build.ImageBuildOptions{
		Tags:       []string{helperImage},
		Dockerfile: "Dockerfile",
		Remove:     true,
		Labels: map[string]string{
			"dse.image.kind": "volume-helper",
		},
	})
	if err != nil {
		return fmt.Errorf("build helper image %q: %w", helperImage, err)
	}
	defer resp.Body.Close()

	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return fmt.Errorf("read helper image build output for %q: %w", helperImage, err)
	}

	return nil
}

func (p *Provider) VolumeExists(name string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	_, ok := p.volumes[name]
	return ok
}
