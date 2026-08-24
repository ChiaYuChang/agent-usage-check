package cache

import (
	"path/filepath"
	"testing"
	"time"

	"agent-usage-check/internal/provider"
)

func TestSaveAndLoadCache(t *testing.T) {
	tempDir := t.TempDir()
	cachePath := filepath.Join(tempDir, "cache.json")

	st := &provider.UsageStatus{
		Provider: "openai",
		Primary:  &provider.Window{RemainingPercent: new(80)},
		Credits:  []provider.Credit{{Status: "available", ExpiresAt: time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)}},
	}

	if err := Save(cachePath, st); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loadedSt, _, err := Load(cachePath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	val, ok := provider.Percentage(loadedSt.Primary)
	if !ok || val != 80 {
		t.Fatalf("Percentage = (%d, %t), want (80, true)", val, ok)
	}

	if len(loadedSt.Credits) != 1 || loadedSt.Credits[0].ExpiresAt != time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("unexpected credits: %+v", loadedSt.Credits)
	}
}
