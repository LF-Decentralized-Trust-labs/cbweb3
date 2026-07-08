// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/dockervolume"
)

// ErrVolumeFileNotFound is returned by readVolumeFile when path does not exist
// inside the volume (mirrors os.ErrNotExist for host-file callers migrating here).
var ErrVolumeFileNotFound = dockervolume.ErrNotFound

// writeVolumeFile, readVolumeFile and volumeFileExists are thin wrappers around
// engine/dockervolume, kept as package-local names since most orchestrator
// steps were written against them before the helper was extracted into its own
// package (bundle.EmitBundle also needs it, hence the extraction).
func writeVolumeFile(ctx context.Context, volume, filePath string, content []byte, mode string) error {
	return dockervolume.WriteFile(ctx, volume, filePath, content, mode)
}

func readVolumeFile(ctx context.Context, volume, filePath string) ([]byte, error) {
	return dockervolume.ReadFile(ctx, volume, filePath)
}

func volumeFileExists(ctx context.Context, volume, filePath string) (bool, error) {
	return dockervolume.FileExists(ctx, volume, filePath)
}
