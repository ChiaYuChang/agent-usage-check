package formatter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-usage-check/internal/provider"
)

func intPtr(v int) *int {
	return &v
}

func TestReportShowsUsageAndCreditExpiry(t *testing.T) {
	now := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	st := &provider.UsageStatus{
		Provider: "openai",
		Primary:  &provider.Window{UsedPercent: intPtr(3)},
		Credits: []provider.Credit{{
			Status:    "available",
			ExpiresAt: time.Date(2026, time.August, 13, 1, 31, 24, 0, time.UTC),
		}},
	}
	report := Report(st, now, DisplayOptions{RemainingChar: "#", UsedChar: "-"})
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

func TestReportAgy(t *testing.T) {
	now := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	reset5h := now.Add(2 * time.Hour)
	reset1w := now.Add(48 * time.Hour)
	st := &provider.UsageStatus{
		Provider: "agy",
		Groups: []provider.Group{
			{
				Name:        "Gemini Models",
				Description: "Gemini Flash, Gemini Pro",
				Windows: map[string]*provider.Window{
					"5h": {
						RemainingPercent: intPtr(50),
						ResetTime:        &reset5h,
					},
					"1w": {
						RemainingPercent: intPtr(80),
						ResetTime:        &reset1w,
					},
				},
			},
		},
	}
	report := Report(st, now, DisplayOptions{RemainingChar: "#", UsedChar: "-"})
	if !strings.Contains(report, "Antigravity (AGY) Usage") {
		t.Errorf("report missing AGY header:\n%s", report)
	}
	if !strings.Contains(report, "[Gemini Models]") {
		t.Errorf("report missing group name:\n%s", report)
	}
	if !strings.Contains(report, "5h: 50% remaining, 50% used") {
		t.Errorf("report missing 5h details:\n%s", report)
	}
}

func TestAshellStatus(t *testing.T) {
	st := &provider.UsageStatus{
		Primary:   &provider.Window{RemainingPercent: intPtr(97)},
		Secondary: &provider.Window{RemainingPercent: intPtr(40)},
	}
	out := AshellStatus(st, nil, DisplayOptions{
		Mode: "remaining",
		Icon: "󰫣",
	})
	if out.Text != "󰫣  5h: 97%  1w: 40%" || out.Alt != "available" {
		t.Errorf("unexpected ashell output: %+v", out)
	}
}

func TestStatusJSON(t *testing.T) {
	st := &provider.UsageStatus{
		Provider:  "agy",
		Primary:   &provider.Window{RemainingPercent: intPtr(97)},
		Secondary: &provider.Window{RemainingPercent: intPtr(40)},
	}
	out := StatusJSON(st, nil, DisplayOptions{
		Mode: "used",
		Icon: "󰫣",
		JSON: true,
	})
	if out.Name != "agy" {
		t.Errorf("out.Name = %q, want agy", out.Name)
	}
	if out.Usage["5h"] != 3 {
		t.Errorf("out.Usage[\"5h\"] = %v, want 3", out.Usage["5h"])
	}
	if out.Usage["1w"] != 60 {
		t.Errorf("out.Usage[\"1w\"] = %v, want 60", out.Usage["1w"])
	}
}

func TestWriteStatusJSON(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "nested", "status.json")
	st := &provider.UsageStatus{
		Primary: &provider.Window{RemainingPercent: intPtr(74)},
	}

	if err := WriteStatusJSON(outputPath, st, nil, DisplayOptions{
		Mode: "remaining",
		Icon: "codex",
	}); err != nil {
		t.Fatalf("WriteStatusJSON failed: %v", err)
	}

	contents, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output JSON: %v", err)
	}
	var out AshellOutput
	if err := json.Unmarshal(contents, &out); err != nil {
		t.Fatalf("failed to unmarshal output JSON: %v", err)
	}

	if out.Text != "codex  5h: 74%  1w: N/A" || out.Alt != "available" {
		t.Fatalf("unexpected output metadata: %+v", out)
	}
}
