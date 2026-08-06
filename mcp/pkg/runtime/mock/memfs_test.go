// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package mock

import (
	"context"
	"testing"

	"github.com/boschglobal/dse.agents/mcp/pkg/volume"
)

func TestMockRuntimeMemFSReadWriteAndList(t *testing.T) {
	ctx := context.Background()
	const sessionID = "test-session"

	rt := NewMockRuntime()

	vol, err := rt.CreateVolume(ctx, sessionID, volume.Spec{
		Name:      "workspace",
		Kind:      volume.KindMemFS,
		MountPath: "/workspace",
	}, nil)
	if err != nil {
		t.Fatalf("CreateVolume() error = %v", err)
	}

	original := []byte("hello memfs")
	if err := rt.WriteVolumeFile(ctx, sessionID, vol, "alpha.txt", original); err != nil {
		t.Fatalf("WriteVolumeFile(alpha.txt) error = %v", err)
	}

	// Verify writes are copied into the volume.
	original[0] = 'H'

	got, err := rt.ReadVolumeFile(ctx, sessionID, vol, "alpha.txt")
	if err != nil {
		t.Fatalf("ReadVolumeFile(alpha.txt) error = %v", err)
	}
	if string(got) != "hello memfs" {
		t.Fatalf("ReadVolumeFile(alpha.txt) = %q, want %q", got, "hello memfs")
	}

	// Verify reads return a defensive copy.
	got[0] = 'H'

	gotAgain, err := rt.ReadVolumeFile(ctx, sessionID, vol, "alpha.txt")
	if err != nil {
		t.Fatalf("ReadVolumeFile(alpha.txt) second read error = %v", err)
	}
	if string(gotAgain) != "hello memfs" {
		t.Fatalf("second ReadVolumeFile(alpha.txt) = %q, want %q", gotAgain, "hello memfs")
	}

	writes := map[string]string{
		"beta.txt":          "beta",
		"dir/zeta.txt":      "zeta",
		"dir/sub/alpha.txt": "nested alpha",
		"dir/sub/beta.txt":  "nested beta",
	}

	for path, data := range writes {
		if err := rt.WriteVolumeFile(ctx, sessionID, vol, path, []byte(data)); err != nil {
			t.Fatalf("WriteVolumeFile(%q) error = %v", path, err)
		}
	}

	rootFiles, err := rt.ListVolumeFiles(ctx, sessionID, vol, ".")
	if err != nil {
		t.Fatalf("ListVolumeFiles(.) error = %v", err)
	}

	assertFileInfos(t, rootFiles, []wantFileInfo{
		{Path: "dir", Name: "dir", IsDir: true},
		{Path: "alpha.txt", Name: "alpha.txt", Size: int64(len("hello memfs"))},
		{Path: "beta.txt", Name: "beta.txt", Size: int64(len("beta"))},
	})

	dirFiles, err := rt.ListVolumeFiles(ctx, sessionID, vol, "dir")
	if err != nil {
		t.Fatalf("ListVolumeFiles(dir) error = %v", err)
	}

	assertFileInfos(t, dirFiles, []wantFileInfo{
		{Path: "dir/sub", Name: "sub", IsDir: true},
		{Path: "dir/zeta.txt", Name: "zeta.txt", Size: int64(len("zeta"))},
	})

	subDirFiles, err := rt.ListVolumeFiles(ctx, sessionID, vol, "dir/sub")
	if err != nil {
		t.Fatalf("ListVolumeFiles(dir/sub) error = %v", err)
	}

	assertFileInfos(t, subDirFiles, []wantFileInfo{
		{Path: "dir/sub/alpha.txt", Name: "alpha.txt", Size: int64(len("nested alpha"))},
		{Path: "dir/sub/beta.txt", Name: "beta.txt", Size: int64(len("nested beta"))},
	})
}

func TestMockRuntimeMemFSDeleteInvalidatesVolume(t *testing.T) {
	ctx := context.Background()
	const sessionID = "test-session"

	rt := NewMockRuntime()

	vol, err := rt.CreateVolume(ctx, sessionID, volume.Spec{
		Name:      "workspace",
		Kind:      volume.KindMemFS,
		MountPath: "/workspace",
	}, nil)
	if err != nil {
		t.Fatalf("CreateVolume() error = %v", err)
	}

	if err := rt.WriteVolumeFile(ctx, sessionID, vol, "file.txt", []byte("content")); err != nil {
		t.Fatalf("WriteVolumeFile() error = %v", err)
	}

	if err := rt.DeleteVolume(ctx, sessionID, vol, nil); err != nil {
		t.Fatalf("DeleteVolume() error = %v", err)
	}

	if _, err := rt.ReadVolumeFile(ctx, sessionID, vol, "file.txt"); err == nil {
		t.Fatal("ReadVolumeFile() after DeleteVolume() error = nil, want error")
	}

	if _, err := rt.ListVolumeFiles(ctx, sessionID, vol, "."); err == nil {
		t.Fatal("ListVolumeFiles() after DeleteVolume() error = nil, want error")
	}

	if err := rt.WriteVolumeFile(ctx, sessionID, vol, "file.txt", []byte("new")); err == nil {
		t.Fatal("WriteVolumeFile() after DeleteVolume() error = nil, want error")
	}
}

func TestMockRuntimeMemFSReadOnlyRejectsWrites(t *testing.T) {
	ctx := context.Background()
	const sessionID = "test-session"

	rt := NewMockRuntime()

	vol, err := rt.CreateVolume(ctx, sessionID, volume.Spec{
		Name:      "readonly",
		Kind:      volume.KindMemFS,
		MountPath: "/readonly",
		ReadOnly:  true,
	}, nil)
	if err != nil {
		t.Fatalf("CreateVolume() error = %v", err)
	}

	if err := rt.WriteVolumeFile(ctx, sessionID, vol, "file.txt", []byte("content")); err == nil {
		t.Fatal("WriteVolumeFile() on read-only volume error = nil, want error")
	}
}

func TestMockRuntimeMemFSRejectsUnsafePaths(t *testing.T) {
	ctx := context.Background()
	const sessionID = "test-session"

	rt := NewMockRuntime()

	vol, err := rt.CreateVolume(ctx, sessionID, volume.Spec{
		Name:      "workspace",
		Kind:      volume.KindMemFS,
		MountPath: "/workspace",
	}, nil)
	if err != nil {
		t.Fatalf("CreateVolume() error = %v", err)
	}

	if err := rt.WriteVolumeFile(ctx, sessionID, vol, "../escape.txt", []byte("bad")); err == nil {
		t.Fatal("WriteVolumeFile() with unsafe path error = nil, want error")
	}

	if _, err := rt.ReadVolumeFile(ctx, sessionID, vol, "../escape.txt"); err == nil {
		t.Fatal("ReadVolumeFile() with unsafe path error = nil, want error")
	}

	if _, err := rt.ListVolumeFiles(ctx, sessionID, vol, "../"); err == nil {
		t.Fatal("ListVolumeFiles() with unsafe path error = nil, want error")
	}
}

type wantFileInfo struct {
	Path  string
	Name  string
	IsDir bool
	Size  int64
}

func assertFileInfos(t *testing.T, got []volume.FileInfo, want []wantFileInfo) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("len(FileInfo) = %d, want %d\ngot = %#v", len(got), len(want), got)
	}

	for i := range want {
		if got[i].Path != want[i].Path {
			t.Errorf("FileInfo[%d].Path = %q, want %q", i, got[i].Path, want[i].Path)
		}
		if got[i].Name != want[i].Name {
			t.Errorf("FileInfo[%d].Name = %q, want %q", i, got[i].Name, want[i].Name)
		}
		if got[i].IsDir != want[i].IsDir {
			t.Errorf("FileInfo[%d].IsDir = %v, want %v", i, got[i].IsDir, want[i].IsDir)
		}

		if want[i].IsDir {
			continue
		}

		if got[i].Size != want[i].Size {
			t.Errorf("FileInfo[%d].Size = %d, want %d", i, got[i].Size, want[i].Size)
		}
		if got[i].Mode == "" {
			t.Errorf("FileInfo[%d].Mode is empty", i)
		}
		if got[i].Modified == "" {
			t.Errorf("FileInfo[%d].Modified is empty", i)
		}
	}
}

func TestMockRuntimeCreateVolumeRejectsDuplicateName(t *testing.T) {
	ctx := context.Background()
	const sessionID = "test-session"

	rt := NewMockRuntime()

	_, err := rt.CreateVolume(ctx, sessionID, volume.Spec{
		Name:      "workspace",
		Kind:      volume.KindMemFS,
		MountPath: "/workspace",
	}, nil)
	if err != nil {
		t.Fatalf("CreateVolume() first volume error = %v", err)
	}

	_, err = rt.CreateVolume(ctx, sessionID, volume.Spec{
		Name:      "workspace",
		Kind:      volume.KindMemFS,
		MountPath: "/different-workspace",
	}, nil)
	if err == nil {
		t.Fatal("CreateVolume() with duplicate name error = nil, want error")
	}
}

func TestMockRuntimeCreateVolumeRejectsDuplicateMountPath(t *testing.T) {
	ctx := context.Background()
	const sessionID = "test-session"

	rt := NewMockRuntime()

	_, err := rt.CreateVolume(ctx, sessionID, volume.Spec{
		Name:      "workspace",
		Kind:      volume.KindMemFS,
		MountPath: "/workspace",
	}, nil)
	if err != nil {
		t.Fatalf("CreateVolume() first volume error = %v", err)
	}

	_, err = rt.CreateVolume(ctx, sessionID, volume.Spec{
		Name:      "different-workspace",
		Kind:      volume.KindMemFS,
		MountPath: "/workspace",
	}, nil)
	if err == nil {
		t.Fatal("CreateVolume() with duplicate mount path error = nil, want error")
	}
}

func TestMockRuntimeDeleteVolumeIgnoresNonExistingVolume(t *testing.T) {
	ctx := context.Background()
	const sessionID = "test-session"

	rt := NewMockRuntime()

	vol, err := rt.CreateVolume(ctx, sessionID, volume.Spec{
		Name:      "workspace",
		Kind:      volume.KindMemFS,
		MountPath: "/workspace",
	}, nil)
	if err != nil {
		t.Fatalf("CreateVolume() error = %v", err)
	}

	if err := rt.DeleteVolume(ctx, sessionID, vol, nil); err != nil {
		t.Fatalf("DeleteVolume() first delete error = %v", err)
	}

	if err := rt.DeleteVolume(ctx, sessionID, vol, nil); err != nil {
		t.Fatalf("DeleteVolume() for non-existing volume error = %v, want nil", err)
	}
}

func TestMockRuntimeReadWriteListAfterDeleteFail(t *testing.T) {
	ctx := context.Background()
	const sessionID = "test-session"

	rt, vol := newTestMemFSVolume(t, ctx, sessionID)

	if err := rt.WriteVolumeFile(ctx, sessionID, vol, "file.txt", []byte("content")); err != nil {
		t.Fatalf("WriteVolumeFile() before delete error = %v", err)
	}

	if err := rt.DeleteVolume(ctx, sessionID, vol, nil); err != nil {
		t.Fatalf("DeleteVolume() error = %v", err)
	}

	if _, err := rt.ReadVolumeFile(ctx, sessionID, vol, "file.txt"); err == nil {
		t.Fatal("ReadVolumeFile() after DeleteVolume() error = nil, want error")
	}

	if err := rt.WriteVolumeFile(ctx, sessionID, vol, "file.txt", []byte("new content")); err == nil {
		t.Fatal("WriteVolumeFile() after DeleteVolume() error = nil, want error")
	}

	if _, err := rt.ListVolumeFiles(ctx, sessionID, vol, "."); err == nil {
		t.Fatal("ListVolumeFiles() after DeleteVolume() error = nil, want error")
	}
}

func TestMockRuntimeReadNonExistingFileFails(t *testing.T) {
	ctx := context.Background()
	const sessionID = "test-session"

	rt, vol := newTestMemFSVolume(t, ctx, sessionID)

	if _, err := rt.ReadVolumeFile(ctx, sessionID, vol, "missing.txt"); err == nil {
		t.Fatal("ReadVolumeFile() for missing file error = nil, want error")
	}
}

func TestMockRuntimeListNonExistingDirectoryReturnsEmptyList(t *testing.T) {
	ctx := context.Background()
	const sessionID = "test-session"

	rt, vol := newTestMemFSVolume(t, ctx, sessionID)

	if err := rt.WriteVolumeFile(ctx, sessionID, vol, "existing/file.txt", []byte("content")); err != nil {
		t.Fatalf("WriteVolumeFile() error = %v", err)
	}

	files, err := rt.ListVolumeFiles(ctx, sessionID, vol, "missing")
	if err != nil {
		t.Fatalf("ListVolumeFiles() for missing directory error = %v, want nil", err)
	}
	if len(files) != 0 {
		t.Fatalf("ListVolumeFiles() for missing directory returned %d files, want 0: %#v", len(files), files)
	}
}

func TestMockRuntimeWriteVolumeFileRejectsInvalidPaths(t *testing.T) {
	ctx := context.Background()
	const sessionID = "test-session"

	rt, vol := newTestMemFSVolume(t, ctx, sessionID)

	tests := []struct {
		name string
		path string
	}{
		{
			name: "empty path",
			path: "",
		},
		{
			name: "current directory",
			path: ".",
		},
		{
			name: "absolute path",
			path: "/escape.txt",
		},
		{
			name: "parent directory",
			path: "../escape.txt",
		},
		{
			name: "nested parent directory escapes root",
			path: "dir/../../escape.txt",
		},
		{
			name: "parent directory after clean escapes root",
			path: "dir/sub/../../../escape.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := rt.WriteVolumeFile(ctx, sessionID, vol, tt.path, []byte("content")); err == nil {
				t.Fatalf("WriteVolumeFile(%q) error = nil, want error", tt.path)
			}
		})
	}
}

func TestMockRuntimeReadVolumeFileRejectsInvalidPaths(t *testing.T) {
	ctx := context.Background()
	const sessionID = "test-session"

	rt, vol := newTestMemFSVolume(t, ctx, sessionID)

	if err := rt.WriteVolumeFile(ctx, sessionID, vol, "safe/file.txt", []byte("content")); err != nil {
		t.Fatalf("WriteVolumeFile() setup error = %v", err)
	}

	tests := []struct {
		name string
		path string
	}{
		{
			name: "empty path",
			path: "",
		},
		{
			name: "current directory",
			path: ".",
		},
		{
			name: "absolute path",
			path: "/safe/file.txt",
		},
		{
			name: "parent directory",
			path: "../safe/file.txt",
		},
		{
			name: "nested parent directory escapes root",
			path: "safe/../../file.txt",
		},
		{
			name: "parent directory after clean escapes root",
			path: "safe/sub/../../../file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := rt.ReadVolumeFile(ctx, sessionID, vol, tt.path); err == nil {
				t.Fatalf("ReadVolumeFile(%q) error = nil, want error", tt.path)
			}
		})
	}
}

func TestMockRuntimeListVolumeFilesRejectsEscapingPaths(t *testing.T) {
	ctx := context.Background()
	const sessionID = "test-session"

	rt, vol := newTestMemFSVolume(t, ctx, sessionID)

	if err := rt.WriteVolumeFile(ctx, sessionID, vol, "safe/file.txt", []byte("content")); err != nil {
		t.Fatalf("WriteVolumeFile() setup error = %v", err)
	}

	tests := []struct {
		name string
		path string
	}{
		{
			name: "absolute path",
			path: "/safe",
		},
		{
			name: "parent directory",
			path: "../safe",
		},
		{
			name: "nested parent directory escapes root",
			path: "safe/../../safe",
		},
		{
			name: "parent directory after clean escapes root",
			path: "safe/sub/../../../safe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := rt.ListVolumeFiles(ctx, sessionID, vol, tt.path); err == nil {
				t.Fatalf("ListVolumeFiles(%q) error = nil, want error", tt.path)
			}
		})
	}
}

func TestMockRuntimeListVolumeFilesAllowsRootPaths(t *testing.T) {
	ctx := context.Background()
	const sessionID = "test-session"

	rt, vol := newTestMemFSVolume(t, ctx, sessionID)

	if err := rt.WriteVolumeFile(ctx, sessionID, vol, "file.txt", []byte("content")); err != nil {
		t.Fatalf("WriteVolumeFile() setup error = %v", err)
	}

	tests := []struct {
		name string
		path string
	}{
		{
			name: "empty path",
			path: "",
		},
		{
			name: "current directory",
			path: ".",
		},
		{
			name: "clean current directory",
			path: "./",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := rt.ListVolumeFiles(ctx, sessionID, vol, tt.path)
			if err != nil {
				t.Fatalf("ListVolumeFiles(%q) error = %v, want nil", tt.path, err)
			}
			if len(files) != 1 {
				t.Fatalf("ListVolumeFiles(%q) returned %d files, want 1: %#v", tt.path, len(files), files)
			}
			if files[0].Path != "file.txt" {
				t.Fatalf("ListVolumeFiles(%q)[0].Path = %q, want %q", tt.path, files[0].Path, "file.txt")
			}
		})
	}
}

func newTestMemFSVolume(t *testing.T, ctx context.Context, sessionID string) (*MockRuntime, *volume.Volume) {
	t.Helper()

	rt := NewMockRuntime()

	vol, err := rt.CreateVolume(ctx, sessionID, volume.Spec{
		Name:      "workspace",
		Kind:      volume.KindMemFS,
		MountPath: "/workspace",
	}, nil)
	if err != nil {
		t.Fatalf("CreateVolume() error = %v", err)
	}

	return rt, vol
}
