package agy

import (
	"context"
	"testing"
	"time"

	"agent-usage-check/internal/provider"
)

const sampleAgyJSON = `{
  "conversation_id": "",
  "status": "SUCCESS",
  "response": "Gemini Models\tWeekly Limit Remaining\t66%\t2026-08-27T12:39:39Z\n",
  "duration_seconds": 0,
  "num_turns": 0,
  "command": {
    "name": "usage",
    "data": {
      "description": "Rate limits description",
      "groups": [
        {
          "name": "Gemini Models",
          "description": "Gemini Flash, Gemini Pro",
          "buckets": [
            {
              "id": "gemini-weekly",
              "name": "Weekly Limit Remaining",
              "window": "weekly",
              "remaining_fraction": 0.6559,
              "reset_time": "2026-08-27T12:39:39Z"
            },
            {
              "id": "gemini-5h",
              "name": "Five Hour Limit Remaining",
              "window": "5h",
              "remaining_fraction": 0.1862,
              "reset_time": "2026-08-24T18:40:55Z"
            }
          ]
        },
        {
          "name": "Claude and GPT models",
          "description": "Claude Opus, Claude Sonnet",
          "buckets": [
            {
              "id": "3p-weekly",
              "name": "Weekly Limit Remaining",
              "window": "weekly",
              "remaining_fraction": 0.6606,
              "reset_time": "2026-08-26T12:22:28Z"
            },
            {
              "id": "3p-5h",
              "name": "Five Hour Limit Remaining",
              "window": "5h",
              "remaining_fraction": 1.0,
              "reset_time": "2026-08-24T21:33:08Z"
            }
          ]
        }
      ]
    }
  }
}`

func TestAgyFetchUsage(t *testing.T) {
	p := New("agy", "")
	p.ExecCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte(sampleAgyJSON), nil
	}

	st, err := p.FetchUsage(context.Background())
	if err != nil {
		t.Fatalf("FetchUsage failed: %v", err)
	}

	if st.Provider != "agy" {
		t.Errorf("Provider = %q, want agy", st.Provider)
	}

	if len(st.Groups) != 2 {
		t.Fatalf("len(Groups) = %d, want 2", len(st.Groups))
	}

	// Primary window (5h) for first group (Gemini Models)
	rem5h, ok := provider.Percentage(st.Primary)
	if !ok || rem5h != 19 {
		t.Errorf("Primary 5h = (%d, %t), want (19, true)", rem5h, ok)
	}

	// Secondary window (1w) for first group
	rem1w, ok := provider.Percentage(st.Secondary)
	if !ok || rem1w != 66 {
		t.Errorf("Secondary 1w = (%d, %t), want (66, true)", rem1w, ok)
	}
}

func TestAgyFetchUsageWithTargetGroup(t *testing.T) {
	p := New("agy", "Claude")
	p.ExecCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte(sampleAgyJSON), nil
	}

	st, err := p.FetchUsage(context.Background())
	if err != nil {
		t.Fatalf("FetchUsage failed: %v", err)
	}

	// For Claude group, 5h remaining fraction is 1.0 -> 100%
	rem5h, ok := provider.Percentage(st.Primary)
	if !ok || rem5h != 100 {
		t.Errorf("Primary 5h = (%d, %t), want (100, true)", rem5h, ok)
	}
}

func TestParseBucket(t *testing.T) {
	now := time.Date(2026, 8, 24, 18, 0, 0, 0, time.UTC)
	b := bucketData{
		Window:            "5h",
		RemainingFraction: 0.5,
		ResetTime:         "2026-08-24T19:00:00Z",
	}
	win := parseBucket(b, now)
	if win.RemainingPercent == nil || *win.RemainingPercent != 50 {
		t.Fatalf("RemainingPercent = %v, want 50", win.RemainingPercent)
	}
	if win.ResetAfter == nil || *win.ResetAfter != 1*time.Hour {
		t.Fatalf("ResetAfter = %v, want 1h", win.ResetAfter)
	}
}
