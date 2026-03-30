package embedding

import (
	"strings"
	"testing"
)

func TestApproxTokenCount(t *testing.T) {
	tests := []struct {
		text string
		min  int
		max  int
	}{
		{"hello world", 2, 4},
		{"The quick brown fox jumps over the lazy dog.", 10, 16},
		{"", 0, 0},
		{"single", 1, 2},
	}

	for _, tt := range tests {
		got := ApproxTokenCount(tt.text)
		if got < tt.min || got > tt.max {
			t.Errorf("ApproxTokenCount(%q) = %d, expected between %d and %d", tt.text, got, tt.min, tt.max)
		}
	}
}

func TestModelCacheDir(t *testing.T) {
	dir, err := ModelCacheDir("nomic-ai/nomic-embed-text-v1.5")
	if err != nil {
		t.Fatalf("ModelCacheDir: %v", err)
	}
	if dir == "" {
		t.Fatal("expected non-empty dir")
	}
	if !strings.Contains(dir, "nomic-ai--nomic-embed-text-v1.5") {
		t.Errorf("expected dir to contain sanitized model name, got %q", dir)
	}
	t.Logf("cache dir: %s", dir)
}

func TestL2Normalize(t *testing.T) {
	v := Vector{3.0, 4.0}
	normalized := l2Normalize(v)

	// 3/5 = 0.6, 4/5 = 0.8
	if abs(normalized[0]-0.6) > 0.001 || abs(normalized[1]-0.8) > 0.001 {
		t.Errorf("expected [0.6, 0.8], got %v", normalized)
	}

	// Verify unit length.
	var norm float64
	for _, x := range normalized {
		norm += float64(x) * float64(x)
	}
	if abs(float32(norm)-1.0) > 0.001 {
		t.Errorf("expected unit norm, got %f", norm)
	}
}

func TestL2NormalizeZero(t *testing.T) {
	v := Vector{0.0, 0.0, 0.0}
	normalized := l2Normalize(v)
	for i, x := range normalized {
		if x != 0.0 {
			t.Errorf("expected 0 at index %d, got %f", i, x)
		}
	}
}

func abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}
