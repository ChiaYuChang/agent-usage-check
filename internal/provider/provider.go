package provider

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const NotApplicable = "N/A"

// Window represents a rate limit window (e.g., 5h or 1w).
type Window struct {
	UsedPercent      *int           `json:"used_percent,omitempty"`
	RemainingPercent *int           `json:"remaining_percent,omitempty"`
	ResetAfter       *time.Duration `json:"reset_after,omitempty"`
	ResetTime        *time.Time     `json:"reset_time,omitempty"`
}

// Group represents a logical grouping of models/limits (e.g. "Gemini Models", "OpenAI").
type Group struct {
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Windows     map[string]*Window `json:"windows,omitempty"`
	Primary     *Window            `json:"primary,omitempty"`
	Secondary   *Window            `json:"secondary,omitempty"`
}

// Credit represents reload tokens / reset credits.
type Credit struct {
	ID        string    `json:"id,omitempty"`
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expires_at"`
}

// UsageStatus represents standard usage information across providers.
type UsageStatus struct {
	Provider  string    `json:"provider"`
	UpdatedAt time.Time `json:"updated_at"`
	Groups    []Group   `json:"groups,omitempty"`
	Primary   *Window   `json:"primary,omitempty"`
	Secondary *Window   `json:"secondary,omitempty"`
	Credits   []Credit  `json:"credits,omitempty"`
}

// Provider is the interface all agent usage providers must implement.
type Provider interface {
	Name() string
	FetchUsage(ctx context.Context) (*UsageStatus, error)
}

// Percentage extracts the remaining percentage and boolean status from a Window.
func Percentage(w *Window) (int, bool) {
	if w == nil {
		return 0, false
	}
	if w.RemainingPercent != nil {
		return *w.RemainingPercent, true
	}
	if w.UsedPercent != nil {
		return 100 - *w.UsedPercent, true
	}
	return 0, false
}

// Bar renders an ASCII progress bar for a Window.
func Bar(w *Window, width int, remainingChar, usedChar string) string {
	remaining, ok := Percentage(w)
	if !ok {
		return strings.Repeat("-", width)
	}
	filled := remaining * width / 100
	if filled > width {
		filled = width
	} else if filled < 0 {
		filled = 0
	}
	return strings.Repeat(remainingChar, filled) + strings.Repeat(usedChar, width-filled)
}

// DurationString formats a time.Duration into a human-readable string (e.g. "2d 10h 30m").
func DurationString(duration time.Duration) string {
	if duration < time.Minute {
		return "less than 1m"
	}
	minutes := int(duration.Round(time.Minute).Minutes())
	days := minutes / (24 * 60)
	minutes %= 24 * 60
	hours := minutes / 60
	minutes %= 60
	parts := make([]string, 0, 3)
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 || days > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	parts = append(parts, fmt.Sprintf("%dm", minutes))
	return strings.Join(parts, " ")
}
