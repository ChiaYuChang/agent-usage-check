package formatter

import (
	"encoding/json"
	"errors"
	"fmt"
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
		"Query time: 2026-08-01 12:00",
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
	if !strings.Contains(report, "Query time: 2026-08-01 12:00") {
		t.Errorf("report missing query time:\n%s", report)
	}
	if !strings.Contains(report, "[Gemini Models]") {
		t.Errorf("report missing group name:\n%s", report)
	}
	if !strings.Contains(report, "5h: 50% remaining, 50% used") {
		t.Errorf("report missing 5h details:\n%s", report)
	}
}

func TestReportOpencodeGo(t *testing.T) {
	now := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	st := &provider.UsageStatus{
		Provider:  "opencode-go",
		Primary:   &provider.Window{RemainingPercent: intPtr(93)},
		Secondary: &provider.Window{RemainingPercent: intPtr(87)},
		Groups: []provider.Group{
			{
				Name: "OpenCode Go",
				Windows: map[string]*provider.Window{
					"rolling": {RemainingPercent: intPtr(93)},
					"weekly":  {RemainingPercent: intPtr(87)},
					"monthly": {RemainingPercent: intPtr(99)},
				},
			},
		},
	}
	report := Report(st, now, DisplayOptions{RemainingChar: "#", UsedChar: "-"})
	if !strings.Contains(report, "OpenCode Go Usage") {
		t.Errorf("report missing OpenCode Go header:\n%s", report)
	}
	if !strings.Contains(report, "Query time: 2026-08-01 12:00") {
		t.Errorf("report missing query time:\n%s", report)
	}
	if !strings.Contains(report, "5h: 93% remaining, 7% used") {
		t.Errorf("report missing 5h details:\n%s", report)
	}
	if !strings.Contains(report, "1w: 87% remaining, 13% used") {
		t.Errorf("report missing 1w details:\n%s", report)
	}
	if !strings.Contains(report, "monthly: 99% remaining, 1% used") {
		t.Errorf("report missing monthly details:\n%s", report)
	}
}

func TestAshellStatus(t *testing.T) {
	st := &provider.UsageStatus{
		Primary:   &provider.Window{RemainingPercent: intPtr(97)},
		Secondary: &provider.Window{RemainingPercent: intPtr(40)},
	}
	const icon = "󰫣"
	out := AshellStatus(st, nil, DisplayOptions{
		Mode: "remaining",
		Icon: icon,
	})
	want := fmt.Sprintf(DefaultTextFormat, icon, " 97%", " 40%")
	if out.Text != want || out.Alt != "available" {
		t.Errorf("unexpected ashell output: %+v, want text %q", out, want)
	}
}

func TestAshellStatusFixedWidth(t *testing.T) {
	const icon = "󰫣"
	full := AshellStatus(&provider.UsageStatus{
		Primary:   &provider.Window{RemainingPercent: intPtr(100)},
		Secondary: &provider.Window{RemainingPercent: intPtr(7)},
	}, nil, DisplayOptions{Mode: "remaining", Icon: icon}).Text
	na := AshellStatus(nil, errors.New("boom"), DisplayOptions{Mode: "remaining", Icon: icon}).Text

	wantFull := fmt.Sprintf(DefaultTextFormat, icon, "100%", "  7%")
	wantNA := fmt.Sprintf(DefaultTextFormat, icon, "N/A ", "N/A ")
	if full != wantFull {
		t.Errorf("unexpected full output: %q, want %q", full, wantFull)
	}
	if na != wantNA {
		t.Errorf("unexpected N/A output: %q, want %q", na, wantNA)
	}
	if len([]rune(full)) != len([]rune(na)) {
		t.Errorf("length mismatch: %q (%d) vs %q (%d)",
			full, len([]rune(full)), na, len([]rune(na)))
	}

	noIcon := AshellStatus(&provider.UsageStatus{
		Primary: &provider.Window{RemainingPercent: intPtr(93)},
	}, nil, DisplayOptions{Mode: "remaining"}).Text
	wantNoIcon := fmt.Sprintf(DefaultTextFormat, "", " 93%", "N/A ")
	if noIcon != wantNoIcon {
		t.Errorf("unexpected no-icon output: %q, want %q", noIcon, wantNoIcon)
	}
}

func TestAshellStatusCustomFormat(t *testing.T) {
	st := &provider.UsageStatus{
		Primary:   &provider.Window{RemainingPercent: intPtr(93)},
		Secondary: &provider.Window{RemainingPercent: intPtr(87)},
	}
	out := AshellStatus(st, nil, DisplayOptions{
		Mode:       "remaining",
		TextFormat: "%s|%s|%s",
	})
	if out.Text != "| 93%| 87%" {
		t.Errorf("unexpected custom format output: %q", out.Text)
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

	if out.Text != fmt.Sprintf(DefaultTextFormat, "codex", " 74%", "N/A ") || out.Alt != "available" {
		t.Fatalf("unexpected output metadata: %+v", out)
	}
}
