// Runtime confirmation for finding VB-1: viperblock's Close, when the
// WAL->chunk consolidation fails but the (separate) state and checkpoint
// saves succeed, still calls RemoveLocalFiles() and deletes the local WAL
// that held the only copy of flushed-but-unconsolidated blocks. Result:
// silent loss of a write the guest was told was flushed.
//
// This is a DEFENSIVE reproduction against our own engine, file backend
// only, entirely under /tmp -- no network, no predastore, no shared state.
//
// Method: wrap the file backend so ONLY FileTypeChunk writes fail (mirrors
// a backend that rejects the large chunk PUT while small state/checkpoint
// writes still land -- e.g. a transient/size-specific failure). Write +
// Flush a verifiable block, Close (which fails the chunk upload), reopen
// through the normal recovery path, and read the block back.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mulgadc/viperblock/types"
	"github.com/mulgadc/viperblock/viperblock"
	"github.com/mulgadc/viperblock/viperblock/backends/file"
)

const blockSizeBytes = 4096

// failChunkBackend embeds the real backend and fails only chunk-object
// writes, leaving config/checkpoint writes working.
type failChunkBackend struct {
	types.Backend
	failChunkWrites bool
}

func (b *failChunkBackend) Write(ft types.FileType, id uint64, h *[]byte, d *[]byte) error {
	if b.failChunkWrites && ft == types.FileTypeChunk {
		return fmt.Errorf("injected chunk-write failure (VB-1 repro)")
	}
	return b.Backend.Write(ft, id, h, d)
}

func (b *failChunkBackend) WriteCtx(ctx context.Context, ft types.FileType, id uint64, h *[]byte, d *[]byte) error {
	if b.failChunkWrites && ft == types.FileTypeChunk {
		return fmt.Errorf("injected chunk-write failure (VB-1 repro)")
	}
	return b.Backend.WriteCtx(ctx, ft, id, h, d)
}

func openVolume(root string, failChunks bool) (*viperblock.VB, *failChunkBackend, error) {
	for _, d := range []string{filepath.Join(root, "backend"), filepath.Join(root, "vb")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, nil, err
		}
	}
	backendConfig := file.FileConfig{VolumeName: "vol0", VolumeSize: 64 * 1024 * 1024, BaseDir: filepath.Join(root, "backend")}
	vbconfig := viperblock.VB{
		VolumeName: "vol0", VolumeSize: 64 * 1024 * 1024, BaseDir: filepath.Join(root, "vb"),
		Cache: viperblock.Cache{Config: viperblock.CacheConfig{Size: 16 * 1024 * 1024}},
	}
	vb, err := viperblock.New(&vbconfig, "file", backendConfig)
	if err != nil {
		return nil, nil, err
	}
	vb.UseShardedWAL = false
	vb.ShardedWAL = nil

	wrapped := &failChunkBackend{Backend: vb.Backend, failChunkWrites: failChunks}
	vb.Backend = wrapped

	if err := vb.Backend.Init(); err != nil {
		return nil, nil, fmt.Errorf("init: %w", err)
	}
	if err := vb.LoadState(); err != nil {
		if e := vb.SaveState(); e != nil {
			return nil, nil, fmt.Errorf("initial SaveState: %w", e)
		}
		if err := vb.LoadState(); err != nil {
			return nil, nil, fmt.Errorf("LoadState: %w", err)
		}
	}
	if err := vb.EnsureVolumeUUID(); err != nil {
		return nil, nil, fmt.Errorf("EnsureVolumeUUID: %w", err)
	}
	if err := vb.LoadLiveCheckpoint(); err != nil {
		return nil, nil, fmt.Errorf("LoadLiveCheckpoint: %w", err)
	}
	if err := vb.RecoverLocalWALs(); err != nil {
		return nil, nil, fmt.Errorf("RecoverLocalWALs: %w", err)
	}
	vb.WAL.WallNum.Add(1)
	if err := vb.OpenWAL(&vb.WAL, fmt.Sprintf("%s/%s", vb.WAL.BaseDir, types.GetFilePath(types.FileTypeWALChunk, vb.WAL.WallNum.Load(), vb.GetVolume()))); err != nil {
		return nil, nil, fmt.Errorf("OpenWAL chunk: %w", err)
	}
	if err := vb.OpenWAL(&vb.BlockToObjectWAL, fmt.Sprintf("%s/%s", vb.WAL.BaseDir, types.GetFilePath(types.FileTypeWALBlock, vb.BlockToObjectWAL.WallNum.Load(), vb.GetVolume()))); err != nil {
		return nil, nil, fmt.Errorf("OpenWAL block: %w", err)
	}
	return vb, wrapped, nil
}

func main() {
	root := "/tmp/vb1_repro"
	_ = os.RemoveAll(root)

	// Phase 1: open with chunk writes failing, write + flush a known block.
	vb, _, err := openVolume(root, true)
	if err != nil {
		fmt.Println("open1:", err)
		os.Exit(1)
	}
	payload := make([]byte, blockSizeBytes)
	copy(payload, []byte("VB1-REPRO-FLUSHED-BLOCK-42"))
	if err := vb.WriteAt(42*blockSizeBytes, payload); err != nil {
		fmt.Println("WriteAt:", err)
		os.Exit(1)
	}
	if err := vb.Flush(); err != nil { // block is now durably in the local WAL
		fmt.Println("Flush:", err)
		os.Exit(1)
	}
	// Read-back before Close: proves the write was acknowledged and present.
	if got, err := vb.ReadAt(42*blockSizeBytes, blockSizeBytes); err != nil {
		fmt.Println("pre-close ReadAt:", err)
	} else {
		fmt.Printf("pre-close read of block 42: %q (present)\n", trim(got))
	}

	closeErr := vb.Close()
	fmt.Printf("Close returned: %v\n", closeErr)
	// Did Close delete the local WAL despite the failure?
	walDir := filepath.Join(root, "vb", "vol0", "wal", "chunks")
	entries, _ := os.ReadDir(walDir)
	fmt.Printf("local WAL files remaining after Close: %d (dir %s)\n", len(entries), walDir)

	// Phase 2: reopen normally (chunk writes work now) and read the block.
	vb2, _, err := openVolume(root, false)
	if err != nil {
		fmt.Println("open2:", err)
		os.Exit(1)
	}
	defer vb2.Close()
	got, err := vb2.ReadAt(42*blockSizeBytes, blockSizeBytes)
	switch {
	case err != nil && err.Error() == "zero block":
		fmt.Println("post-reopen read of block 42: ZERO BLOCK (data gone)")
		fmt.Println("CONFIRMED VB-1: a flushed block was lost because Close deleted the local WAL after a failed chunk upload.")
	case err != nil:
		fmt.Println("post-reopen ReadAt error:", err)
	case string(trim(got)) == "VB1-REPRO-FLUSHED-BLOCK-42":
		fmt.Println("post-reopen read of block 42: still present -> NOT confirmed (data survived).")
	default:
		fmt.Printf("post-reopen read of block 42: unexpected content %q\n", trim(got))
	}
}

func trim(b []byte) []byte {
	n := 0
	for n < len(b) && b[n] != 0 {
		n++
	}
	return b[:n]
}
