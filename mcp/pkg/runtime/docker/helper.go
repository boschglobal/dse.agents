// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

func dockerfileBuildContext(dockerfileContent string) (io.ReadCloser, error) {
	var buf bytes.Buffer

	tw := tar.NewWriter(&buf)

	dockerfileBytes := []byte(dockerfileContent)

	if err := tw.WriteHeader(&tar.Header{
		Name: "Dockerfile",
		Mode: 0o644,
		Size: int64(len(dockerfileBytes)),
	}); err != nil {
		return nil, fmt.Errorf("write Dockerfile tar header: %w", err)
	}

	if _, err := tw.Write(dockerfileBytes); err != nil {
		return nil, fmt.Errorf("write Dockerfile tar content: %w", err)
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("close Dockerfile build context: %w", err)
	}

	return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
}

func cleanVolumeRelPath(filePath string) (string, error) {
	cleanPath := path.Clean(strings.ReplaceAll(filePath, "\\", "/"))

	if cleanPath == "." {
		return "", nil
	}

	if path.IsAbs(cleanPath) {
		return "", fmt.Errorf("volume path %q must be relative", filePath)
	}

	if cleanPath == ".." || strings.HasPrefix(cleanPath, "../") {
		return "", fmt.Errorf("volume path %q escapes volume root", filePath)
	}

	return cleanPath, nil
}

func tarArchiveForFile(filePath string, data []byte) (*bytes.Reader, error) {
	var buf bytes.Buffer

	tw := tar.NewWriter(&buf)

	if err := writeTarParentDirs(tw, path.Dir(filePath)); err != nil {
		return nil, err
	}

	header := &tar.Header{
		Name: filePath,
		Mode: 0o644,
		Size: int64(len(data)),
		Uid:  os.Getuid(),
		Gid:  os.Getgid(),
	}

	if err := tw.WriteHeader(header); err != nil {
		return nil, fmt.Errorf("write tar header for %q: %w", filePath, err)
	}

	if _, err := tw.Write(data); err != nil {
		return nil, fmt.Errorf("write tar content for %q: %w", filePath, err)
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("close tar archive for %q: %w", filePath, err)
	}

	return bytes.NewReader(buf.Bytes()), nil
}

func writeTarParentDirs(tw *tar.Writer, dirPath string) error {
	if dirPath == "." || dirPath == "" {
		return nil
	}

	current := ""

	for _, part := range strings.Split(dirPath, "/") {
		if part == "" {
			continue
		}

		current = path.Join(current, part)

		header := &tar.Header{
			Name:     current,
			Mode:     0o755,
			Typeflag: tar.TypeDir,
			Uid:      os.Getuid(),
			Gid:      os.Getgid(),
		}

		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("write tar directory header for %q: %w", current, err)
		}
	}

	return nil
}
