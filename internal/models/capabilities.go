package models

// ImageModelCapabilities defines the exact supported parameters for an image model.
type ImageModelCapabilities struct {
	ID                    string
	DisplayName           string
	SupportedResolutions  []string
	SupportedAspectRatios []string
	SupportedQualities    []string
	SupportsQuality       bool
	MaxReferences         int
	DefaultResolution     string
	DefaultAspectRatio    string
	DefaultQuality        string
}

// VideoModelCapabilities defines the exact supported parameters for a video model.
type VideoModelCapabilities struct {
	ID                    string
	DisplayName           string
	SupportedResolutions  []string
	SupportedAspectRatios []string
	SupportedDurations    []int
	DefaultResolution     string
	DefaultAspectRatio    string
	DefaultDuration       int
}

const (
	ModelGrokImage20      = "x-ai/grok-imagine-image-2.0"
	ModelGrokImageQuality = "x-ai/grok-imagine-image-quality"
	ModelGrokVideo15      = "x-ai/grok-imagine-video-1.5"
)

var (
	ImageModels = map[string]ImageModelCapabilities{
		ModelGrokImage20: {
			ID:                    ModelGrokImage20,
			DisplayName:           "Grok Imagine Image 2.0",
			SupportedResolutions:  []string{"1K", "2K"},
			SupportedAspectRatios: []string{"1:1", "3:2", "2:3", "4:3", "3:4", "16:9", "9:16"},
			SupportedQualities:    []string{"low", "medium"},
			SupportsQuality:       true,
			MaxReferences:         3,
			DefaultResolution:     "2K",
			DefaultAspectRatio:    "1:1",
			DefaultQuality:        "medium",
		},
		ModelGrokImageQuality: {
			ID:                    ModelGrokImageQuality,
			DisplayName:           "Grok Imagine Image Quality (Legacy)",
			SupportedResolutions:  []string{"1K", "2K"},
			SupportedAspectRatios: []string{"1:1", "3:2", "2:3", "4:3", "3:4", "16:9", "9:16"},
			SupportedQualities:    nil,
			SupportsQuality:       false,
			MaxReferences:         3,
			DefaultResolution:     "2K",
			DefaultAspectRatio:    "1:1",
			DefaultQuality:        "",
		},
	}

	VideoModels = map[string]VideoModelCapabilities{
		ModelGrokVideo15: {
			ID:                    ModelGrokVideo15,
			DisplayName:           "Grok Imagine Video 1.5",
			SupportedResolutions:  []string{"480p", "720p", "1080p"},
			SupportedAspectRatios: []string{"16:9", "9:16", "1:1", "4:3", "3:4", "3:2", "2:3"},
			SupportedDurations:    []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			DefaultResolution:     "720p",
			DefaultAspectRatio:    "16:9",
			DefaultDuration:       6,
		},
	}
)

// GetImageModelCaps returns capabilities for the given image model ID or defaults to Grok 2.0.
func GetImageModelCaps(modelID string) ImageModelCapabilities {
	if caps, ok := ImageModels[modelID]; ok {
		return caps
	}
	return ImageModels[ModelGrokImage20]
}

// GetVideoModelCaps returns capabilities for the given video model ID or defaults to Grok Video 1.5.
func GetVideoModelCaps(modelID string) VideoModelCapabilities {
	if caps, ok := VideoModels[modelID]; ok {
		return caps
	}
	return VideoModels[ModelGrokVideo15]
}

// NormalizeImageSettings ensures that the resolution, aspect ratio, and quality are valid for the selected model.
func NormalizeImageSettings(modelID, resolution, aspectRatio, quality string) (normRes, normRatio, normQuality string) {
	caps := GetImageModelCaps(modelID)

	// Resolution check
	normRes = caps.DefaultResolution
	for _, r := range caps.SupportedResolutions {
		if r == resolution {
			normRes = resolution
			break
		}
	}

	// Aspect ratio check
	normRatio = caps.DefaultAspectRatio
	for _, a := range caps.SupportedAspectRatios {
		if a == aspectRatio {
			normRatio = aspectRatio
			break
		}
	}

	// Quality check
	if !caps.SupportsQuality {
		normQuality = ""
	} else {
		normQuality = caps.DefaultQuality
		for _, q := range caps.SupportedQualities {
			if q == quality {
				normQuality = quality
				break
			}
		}
	}

	return normRes, normRatio, normQuality
}

// NormalizeVideoSettings ensures that the resolution, aspect ratio, and duration are valid for the selected video model.
func NormalizeVideoSettings(modelID, resolution, aspectRatio string, duration int) (normRes, normRatio string, normDuration int) {
	caps := GetVideoModelCaps(modelID)

	normRes = caps.DefaultResolution
	for _, r := range caps.SupportedResolutions {
		if r == resolution {
			normRes = resolution
			break
		}
	}

	normRatio = caps.DefaultAspectRatio
	for _, a := range caps.SupportedAspectRatios {
		if a == aspectRatio {
			normRatio = aspectRatio
			break
		}
	}

	normDuration = caps.DefaultDuration
	for _, d := range caps.SupportedDurations {
		if d == duration {
			normDuration = duration
			break
		}
	}

	return normRes, normRatio, normDuration
}
