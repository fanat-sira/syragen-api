package telegram

import (
	"syragen-apibased/internal/storage"
)

// GenerationMode represents the detected generation mode.
type GenerationMode int

const (
	ModeTextToImage GenerationMode = iota
	ModeImageEdit
	ModeMultiReferenceEdit
	ModeTextToVideo
	ModeImageToVideo
)

// ResolvedContext represents the resolved inputs for a generation request.
type ResolvedContext struct {
	Mode         GenerationMode
	Prompt       string
	SourcePaths  []string                    // Local file paths of source/reference images
	Cleanups     []func()                    // Cleanup functions for temporary downloaded user files
	OriginalMeta *storage.GenerationMetadata // Present if using a previously bot-generated asset
}

// CleanupAll cleans up any temporary user files created during context resolution.
func (rc *ResolvedContext) CleanupAll() {
	for _, cleanup := range rc.Cleanups {
		if cleanup != nil {
			cleanup()
		}
	}
}
