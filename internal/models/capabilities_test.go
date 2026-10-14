package models

import (
	"testing"
)

func TestModelCapabilities(t *testing.T) {
	// Test Grok 2.0 capabilities
	caps20 := GetImageModelCaps(ModelGrokImage20)
	if !caps20.SupportsQuality {
		t.Error("Grok 2.0 should support quality parameter")
	}
	if caps20.MaxReferences != 3 {
		t.Errorf("expected max references 3, got %d", caps20.MaxReferences)
	}

	// Test Legacy Grok Image Quality capabilities
	capsLegacy := GetImageModelCaps(ModelGrokImageQuality)
	if capsLegacy.SupportsQuality {
		t.Error("Legacy Grok Image Quality should NOT support quality parameter")
	}

	// Test normalization on Grok 2.0
	res, ratio, qual := NormalizeImageSettings(ModelGrokImage20, "2K", "16:9", "medium")
	if res != "2K" || ratio != "16:9" || qual != "medium" {
		t.Errorf("expected 2K, 16:9, medium; got %s, %s, %s", res, ratio, qual)
	}

	// Test normalization when invalid values provided
	res, ratio, qual = NormalizeImageSettings(ModelGrokImage20, "8K", "invalid", "ultra")
	if res != "2K" || ratio != "1:1" || qual != "medium" {
		t.Errorf("expected fallback defaults; got %s, %s, %s", res, ratio, qual)
	}

	// Test normalization when switching to legacy model
	res, ratio, qual = NormalizeImageSettings(ModelGrokImageQuality, "2K", "16:9", "medium")
	if qual != "" {
		t.Errorf("expected quality to be stripped for legacy model, got %s", qual)
	}

	// Test Video normalization
	vRes, vRatio, vDur := NormalizeVideoSettings(ModelGrokVideo15, "720p", "16:9", 6)
	if vRes != "720p" || vRatio != "16:9" || vDur != 6 {
		t.Errorf("expected 720p, 16:9, 6; got %s, %s, %d", vRes, vRatio, vDur)
	}

	// Invalid video duration fallback
	vRes, vRatio, vDur = NormalizeVideoSettings(ModelGrokVideo15, "4K", "100:1", 999)
	if vRes != "720p" || vRatio != "16:9" || vDur != 6 {
		t.Errorf("expected fallback video defaults; got %s, %s, %d", vRes, vRatio, vDur)
	}
}
