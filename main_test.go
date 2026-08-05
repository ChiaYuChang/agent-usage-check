package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func intPointer(value int) *int {
	return &value
}

func TestPercentageUsesRemainingValue(t *testing.T) {
	value, ok := percentage(&window{RemainingPercent: intPointer(97)})
	if !ok || value != 97 {
		t.Fatalf("percentage = (%d, %t), want (97, true)", value, ok)
	}
}

func TestPercentageFallsBackToUsedValue(t *testing.T) {
	value, ok := percentage(&window{UsedPercent: intPointer(3)})
	if !ok || value != 97 {
		t.Fatalf("percentage = (%d, %t), want (97, true)", value, ok)
	}
}

func TestReportShowsUsageAndCreditExpiry(t *testing.T) {
	now := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	report := report(status{
		Primary: &window{UsedPercent: intPointer(3)},
		Credits: []credit{{
			Status:    "available",
			ExpiresAt: "2026-08-13T01:31:24Z",
		}},
	}, now, displayOptions{remainingChar: "#", usedChar: "-"})
	for _, expected := range []string{
		"5h: 97% remaining, 3% used",
		"[###########################-]",
		"1w: N/A",
		"- #001: Expires in 11d 13h 31m",
	} {
		if !strings.Contains(report, expected) {
			t.Errorf("report does not contain %q:\n%s", expected, report)
		}
	}
}

func TestBarUsesConfiguredCharacters(t *testing.T) {
	result := bar(&window{RemainingPercent: intPointer(50)}, 4, displayOptions{
		remainingChar: "R",
		usedChar:      "U",
	})
	if result != "RRUU" {
		t.Fatalf("bar = %q, want RRUU", result)
	}
}

func TestSaveAndLoadCache(t *testing.T) {
	tempDir := t.TempDir()
	cachePath := filepath.Join(tempDir, "cache.json")

	st := status{
		Primary: &window{RemainingPercent: intPointer(80)},
		Credits: []credit{{Status: "available", ExpiresAt: "2026-08-10T00:00:00Z"}},
	}

	if err := saveCache(cachePath, st); err != nil {
		t.Fatalf("saveCache failed: %v", err)
	}

	loadedSt, _, err := loadCache(cachePath)
	if err != nil {
		t.Fatalf("loadCache failed: %v", err)
	}

	val, ok := percentage(loadedSt.Primary)
	if !ok || val != 80 {
		t.Fatalf("percentage = (%d, %t), want (80, true)", val, ok)
	}

	if len(loadedSt.Credits) != 1 || loadedSt.Credits[0].ExpiresAt != "2026-08-10T00:00:00Z" {
		t.Fatalf("unexpected credits: %+v", loadedSt.Credits)
	}
}

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("skipping test as home dir is unavailable")
	}

	got := expandPath("~/.config/agent-usage-check")
	want := filepath.Join(home, ".config/agent-usage-check")
	if got != want {
		t.Errorf("expandPath() = %q, want %q", got, want)
	}
}

func TestPrintStatusJSON(t *testing.T) {
	st := status{
		Primary: &window{RemainingPercent: intPointer(97)},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printStatus(st, nil, displayOptions{
		mode: "used",
		icon: "󰫣",
		json: true,
	})

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	var out jsonOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("failed to unmarshal JSON status: %v", err)
	}

	if out.Name != "openai" {
		t.Errorf("out.Name = %q, want openai", out.Name)
	}
	if out.Icon != "󰫣" {
		t.Errorf("out.Icon = %q, want 󰫣", out.Icon)
	}
	if out.Alt != "available" {
		t.Errorf("out.Alt = %q, want available", out.Alt)
	}
	// mode is used, remaining is 97%, so used is 3
	val, ok := out.Usage["5h"]
	if !ok || int(val.(float64)) != 3 {
		t.Errorf("out.Usage[\"5h\"] = %v, want 3", val)
	}
	if out.Usage["1w"] != "N/A" {
		t.Errorf("out.Usage[\"1w\"] = %v, want N/A", out.Usage["1w"])
	}
}
