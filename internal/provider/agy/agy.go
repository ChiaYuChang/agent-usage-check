package agy

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"strings"
	"time"

	"agent-usage-check/internal/provider"
)

const (
	DefaultBinaryPath = "agy"
)

type bucketData struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	Description       string  `json:"description"`
	Window            string  `json:"window"`
	RemainingFraction float64 `json:"remaining_fraction"`
	ResetTime         string  `json:"reset_time"`
}

type groupData struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Buckets     []bucketData `json:"buckets"`
}

type agyResponse struct {
	Status  string `json:"status"`
	Command struct {
		Name string `json:"name"`
		Data struct {
			Description string      `json:"description"`
			Groups      []groupData `json:"groups"`
		} `json:"data"`
	} `json:"command"`
}

type Provider struct {
	BinaryPath  string
	TargetGroup string
	ExecCommand func(ctx context.Context, name string, args ...string) ([]byte, error)
}

func defaultExecCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Output()
}

func New(binPath, targetGroup string) *Provider {
	if binPath == "" {
		binPath = DefaultBinaryPath
	}
	return &Provider{
		BinaryPath:  binPath,
		TargetGroup: targetGroup,
		ExecCommand: defaultExecCommand,
	}
}

func (p *Provider) Name() string {
	return "agy"
}

func parseBucket(b bucketData, now time.Time) *provider.Window {
	rem := int(math.Round(b.RemainingFraction * 100))
	used := 100 - rem
	win := &provider.Window{
		RemainingPercent: &rem,
		UsedPercent:      &used,
	}

	if b.ResetTime != "" {
		var t time.Time
		var err error
		if t, err = time.Parse(time.RFC3339Nano, b.ResetTime); err != nil {
			t, err = time.Parse(time.RFC3339, b.ResetTime)
		}
		if err == nil {
			win.ResetTime = &t
			if t.After(now) {
				dur := t.Sub(now)
				win.ResetAfter = &dur
			} else {
				zero := time.Duration(0)
				win.ResetAfter = &zero
			}
		}
	}
	return win
}

func (p *Provider) FetchUsage(ctx context.Context) (*provider.UsageStatus, error) {
	bin := p.BinaryPath
	if bin == "" {
		bin = DefaultBinaryPath
	}

	execFn := p.ExecCommand
	if execFn == nil {
		execFn = defaultExecCommand
	}

	output, err := execFn(ctx, bin, "-p", "/usage", "-output-format", "json")
	if err != nil {
		return nil, fmt.Errorf("failed to execute %s: %w", bin, err)
	}

	var resp agyResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse agy output: %w", err)
	}

	now := time.Now()
	var groups []provider.Group

	for _, g := range resp.Command.Data.Groups {
		group := provider.Group{
			Name:        g.Name,
			Description: g.Description,
			Windows:     make(map[string]*provider.Window),
		}

		for _, b := range g.Buckets {
			w := parseBucket(b, now)
			wKey := strings.ToLower(b.Window)
			if wKey == "weekly" {
				wKey = "1w"
			}
			group.Windows[wKey] = w

			if wKey == "5h" {
				group.Primary = w
			} else if wKey == "1w" || wKey == "weekly" {
				group.Secondary = w
			}
		}
		groups = append(groups, group)
	}

	if len(groups) == 0 {
		return nil, fmt.Errorf("no usage groups found in agy response")
	}

	var selectedGroup *provider.Group
	if p.TargetGroup != "" {
		target := strings.ToLower(p.TargetGroup)
		for i := range groups {
			if strings.Contains(strings.ToLower(groups[i].Name), target) {
				selectedGroup = &groups[i]
				break
			}
		}
	}

	if selectedGroup == nil {
		// Default to first group
		selectedGroup = &groups[0]
	}

	return &provider.UsageStatus{
		Provider:  p.Name(),
		UpdatedAt: now,
		Groups:    groups,
		Primary:   selectedGroup.Primary,
		Secondary: selectedGroup.Secondary,
		Credits:   nil, // agy doesn't have credit reload tokens
	}, nil
}
