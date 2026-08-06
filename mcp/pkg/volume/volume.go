// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package volume

import (
	"fmt"
	"path"
	"strings"
)

type Kind string

const (
	KindHostPath  Kind = "hostPath"
	KindEphemeral Kind = "ephemeral"
	KindMemFS     Kind = "memfs"
	KindTmpFS     Kind = "tmpfs"
)

type SeedKind string

const (
	SeedKindHTTP      SeedKind = "http"
	SeedKindUploadZip SeedKind = "uploadZip"
)

type Spec struct {
	Name      string `json:"name"`
	Kind      Kind   `json:"kind"`
	MountPath string `json:"mountPath"`
	ReadOnly  bool   `json:"readOnly,omitempty"`

	HostPath  *HostPathSpec  `json:"hostPath,omitempty"`
	Ephemeral *EphemeralSpec `json:"ephemeral,omitempty"`
	MemFS     *MemFSSpec     `json:"memfs,omitempty"`
	TmpFS     *TmpFSSpec     `json:"tmpfs,omitempty"`
}

type HostPathSpec struct {
	Path string `json:"path"`
}

type EphemeralSpec struct {
	Seed *SeedSpec `json:"seed,omitempty"`
}

type MemFSSpec struct {
	Seed *SeedSpec `json:"seed,omitempty"`
}

type TmpFSSpec struct {
	Size string `json:"size,omitempty"`
}

type SeedSpec struct {
	Kind SeedKind `json:"kind"`

	HTTP      *HTTPSeedSpec      `json:"http,omitempty"`
	UploadZip *UploadZipSeedSpec `json:"uploadZip,omitempty"`
}

type HTTPSeedSpec struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Archive bool              `json:"archive,omitempty"`
}

type UploadZipSeedSpec struct {
	UploadID string `json:"uploadId"`
}

type Volume struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionId"`

	Name      string `json:"name"`
	Kind      Kind   `json:"kind"`
	MountPath string `json:"mountPath"`
	ReadOnly  bool   `json:"readOnly,omitempty"`

	// Runtime-specific internal handle.
	Handle any `json:"-"`
}

func CleanPath(p string) (string, error) {
	if p == "" {
		return ".", nil
	}

	clean := path.Clean(strings.ReplaceAll(p, "\\", "/"))
	if clean == "/" || strings.HasPrefix(clean, "/") {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("path escapes volume root")
	}

	return clean, nil
}

func CleanFilePath(p string) (string, error) {
	clean, err := CleanPath(p)
	if err != nil {
		return "", err
	}
	if clean == "." {
		return "", fmt.Errorf("file path must not be empty or current directory")
	}
	return clean, nil
}

func CleanDirPath(p string) (string, error) {
	clean, err := CleanPath(p)
	if err != nil {
		return "", err
	}
	if clean == "." {
		return "", nil
	}
	return clean, nil
}
