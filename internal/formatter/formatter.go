package formatter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-usage-check/internal/cache"
	"agent-usage-check/internal/provider"
)

type DisplayOptions struct {
	Mode          string
	RemainingChar string
	UsedChar      string
	Icon          string
	JSON          bool
}

type AshellOutput struct {
	Text string `json:"text"`
	Alt  string `json:"alt"`
}

type JSONOutput struct {
	Name   string         `json:"name"`
	Icon   string         `json:"icon"`
	Usage  map[string]any `json:"usage"`
	Alt    string         `json:"alt"`
	Groups []any          `json:"groups,omitempty"`
}

func WindowDetails(label string, value *provider.Window, options DisplayOptions) string {
	remaining, ok := provider.Percentage(value)
	if !ok {
		return fmt.Sprintf("%s: %s\n[%s]", label, provider.NotApplicable, provider.Bar(value, 28, options.RemainingChar, options.UsedChar))
	}
	used := 100 - remaining
	details := fmt.Sprintf("%s: %d%% remaining, %d%% used\n[%s]", label, remaining, used, provider.Bar(value, 28, options.RemainingChar, options.UsedChar))
	if value.ResetAfter != nil {
		details += "\nReset in: " + provider.DurationString(*value.ResetAfter)
	} else if value.ResetTime != nil {
		now := time.Now()
		if value.ResetTime.After(now) {
			details += "\nReset in: " + provider.DurationString(value.ResetTime.Sub(now))
		}
	}
	return details
}

func Report(st *provider.UsageStatus, now time.Time, options DisplayOptions) string {
	if st == nil {
		return "No usage data available.\n"
	}

	var output strings.Builder
	if st.Provider == "opencode-go" {
		output.WriteString("OpenCode Go Usage\n\n")
		group := provider.Group{Primary: st.Primary, Secondary: st.Secondary, Windows: map[string]*provider.Window{}}
		if len(st.Groups) > 0 {
			group = st.Groups[0]
		}
		if w, ok := group.Windows["rolling"]; ok {
			output.WriteString(WindowDetails("5h", w, options))
			output.WriteString("\n\n")
		} else if group.Primary != nil {
			output.WriteString(WindowDetails("5h", group.Primary, options))
			output.WriteString("\n\n")
		}
		if w, ok := group.Windows["weekly"]; ok {
			output.WriteString(WindowDetails("1w", w, options))
			output.WriteString("\n\n")
		} else if group.Secondary != nil {
			output.WriteString(WindowDetails("1w", group.Secondary, options))
			output.WriteString("\n\n")
		}
		if w, ok := group.Windows["monthly"]; ok {
			output.WriteString(WindowDetails("monthly", w, options))
			output.WriteString("\n")
		} else if w, ok := group.Windows["1m"]; ok {
			output.WriteString(WindowDetails("monthly", w, options))
			output.WriteString("\n")
		}
		return output.String()
	}
	if st.Provider == "agy" {
		output.WriteString("Antigravity (AGY) Usage\n\n")
		for i, group := range st.Groups {
			if i > 0 {
				output.WriteString("\n----------------------------------------\n\n")
			}
			output.WriteString(fmt.Sprintf("[%s]\n", group.Name))
			if group.Description != "" {
				output.WriteString(fmt.Sprintf("%s\n\n", group.Description))
			}

			if w5h, ok := group.Windows["5h"]; ok {
				output.WriteString(WindowDetails("5h", w5h, options))
				output.WriteString("\n\n")
			} else if group.Primary != nil {
				output.WriteString(WindowDetails("Primary", group.Primary, options))
				output.WriteString("\n\n")
			}

			if w1w, ok := group.Windows["1w"]; ok {
				output.WriteString(WindowDetails("1w", w1w, options))
				output.WriteString("\n")
			} else if group.Secondary != nil {
				output.WriteString(WindowDetails("Secondary", group.Secondary, options))
				output.WriteString("\n")
			}
		}
		return output.String()
	}

	// Default OpenAI / Codex format
	output.WriteString("Codex usage\n\n")
	output.WriteString(WindowDetails("5h", st.Primary, options))
	output.WriteString("\n\n")
	output.WriteString(WindowDetails("1w", st.Secondary, options))
	output.WriteString("\n\nReload Token\n")

	index := 0
	for _, credit := range st.Credits {
		if credit.Status != "available" {
			continue
		}
		index++
		if credit.ExpiresAt.IsZero() {
			fmt.Fprintf(&output, "- #%03d: Expiry unavailable\n", index)
			continue
		}
		fmt.Fprintf(&output, "- #%03d: Expires in %s\n", index, provider.DurationString(credit.ExpiresAt.Sub(now)))
	}
	if index == 0 {
		output.WriteString("- None available\n")
	}
	return output.String()
}

func AshellStatus(st *provider.UsageStatus, err error, options DisplayOptions) AshellOutput {
	summary := fmt.Sprintf("5h: %s  1w: %s", provider.NotApplicable, provider.NotApplicable)
	if options.Icon != "" {
		summary = options.Icon + "  " + summary
	}
	output := AshellOutput{Text: summary, Alt: "error"}
	if err == nil && st != nil {
		primary, primaryOK := provider.Percentage(st.Primary)
		secondary, secondaryOK := provider.Percentage(st.Secondary)
		primaryText := provider.NotApplicable
		secondaryText := provider.NotApplicable
		if primaryOK {
			if options.Mode == "used" {
				primaryText = fmt.Sprintf("%d%%", 100-primary)
			} else {
				primaryText = fmt.Sprintf("%d%%", primary)
			}
		}
		if secondaryOK {
			if options.Mode == "used" {
				secondaryText = fmt.Sprintf("%d%%", 100-secondary)
			} else {
				secondaryText = fmt.Sprintf("%d%%", secondary)
			}
		}
		summary = fmt.Sprintf("5h: %s  1w: %s", primaryText, secondaryText)
		if options.Icon != "" {
			summary = options.Icon + "  " + summary
		}
		output.Text = summary
		if (primaryOK && primary == 0) || (secondaryOK && secondary == 0) {
			output.Alt = "exhausted"
		} else {
			output.Alt = "available"
		}
	}
	return output
}

func StatusJSON(st *provider.UsageStatus, err error, options DisplayOptions) JSONOutput {
	providerName := "openai"
	if st != nil && st.Provider != "" {
		providerName = st.Provider
	}
	out := JSONOutput{
		Name: providerName,
		Icon: options.Icon,
		Usage: map[string]any{
			"5h": provider.NotApplicable,
			"1w": provider.NotApplicable,
		},
		Alt: "error",
	}
	if err != nil || st == nil {
		return out
	}

	primary, primaryOK := provider.Percentage(st.Primary)
	secondary, secondaryOK := provider.Percentage(st.Secondary)
	if primaryOK {
		if options.Mode == "used" {
			out.Usage["5h"] = 100 - primary
		} else {
			out.Usage["5h"] = primary
		}
	}
	if secondaryOK {
		if options.Mode == "used" {
			out.Usage["1w"] = 100 - secondary
		} else {
			out.Usage["1w"] = secondary
		}
	}
	if monthly, monthlyOK := monthlyPercentage(st); monthlyOK {
		if options.Mode == "used" {
			out.Usage["monthly"] = 100 - monthly
		} else {
			out.Usage["monthly"] = monthly
		}
	}
	if (primaryOK && primary == 0) || (secondaryOK && secondary == 0) {
		out.Alt = "exhausted"
	} else {
		out.Alt = "available"
	}

	if len(st.Groups) > 0 {
		for _, g := range st.Groups {
			groupMap := map[string]any{
				"name": g.Name,
			}
			if p, ok := provider.Percentage(g.Primary); ok {
				if options.Mode == "used" {
					groupMap["5h"] = 100 - p
				} else {
					groupMap["5h"] = p
				}
			}
			if s, ok := provider.Percentage(g.Secondary); ok {
				if options.Mode == "used" {
					groupMap["1w"] = 100 - s
				} else {
					groupMap["1w"] = s
				}
			}
			if m, ok := groupMonthlyPercentage(&g); ok {
				if options.Mode == "used" {
					groupMap["monthly"] = 100 - m
				} else {
					groupMap["monthly"] = m
				}
			}
			out.Groups = append(out.Groups, groupMap)
		}
	}

	return out
}

// monthlyPercentage extracts the remaining percentage of the monthly window
// (used by the opencode-go provider) from the first group carrying one.
func monthlyPercentage(st *provider.UsageStatus) (int, bool) {
	if st == nil {
		return 0, false
	}
	for i := range st.Groups {
		if m, ok := groupMonthlyPercentage(&st.Groups[i]); ok {
			return m, true
		}
	}
	return 0, false
}

func groupMonthlyPercentage(g *provider.Group) (int, bool) {
	if g == nil {
		return 0, false
	}
	if w, ok := g.Windows["monthly"]; ok {
		if m, ok := provider.Percentage(w); ok {
			return m, true
		}
	}
	if w, ok := g.Windows["1m"]; ok {
		if m, ok := provider.Percentage(w); ok {
			return m, true
		}
	}
	return 0, false
}

func WriteStatusJSON(path string, st *provider.UsageStatus, err error, options DisplayOptions) error {
	var output any = AshellStatus(st, err, options)
	if options.JSON {
		output = StatusJSON(st, err, options)
	}
	contents, jsonErr := json.Marshal(output)
	if jsonErr != nil {
		return jsonErr
	}
	contents = append(contents, '\n')

	realPath := cache.ExpandPath(path)
	directory := filepath.Dir(realPath)
	if mkErr := os.MkdirAll(directory, 0755); mkErr != nil {
		return mkErr
	}

	temporary, tmpErr := os.CreateTemp(directory, ".agent-usage-check-*.json")
	if tmpErr != nil {
		return tmpErr
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()

	if chErr := temporary.Chmod(0644); chErr != nil {
		_ = temporary.Close()
		return chErr
	}
	if _, wrErr := temporary.Write(contents); wrErr != nil {
		_ = temporary.Close()
		return wrErr
	}
	if clErr := temporary.Close(); clErr != nil {
		return clErr
	}
	return os.Rename(temporaryPath, realPath)
}

func EmitStatus(st *provider.UsageStatus, err error, options DisplayOptions, outputPath string) {
	if outputPath != "" {
		if writeErr := WriteStatusJSON(outputPath, st, err, options); writeErr != nil {
			fmt.Fprintln(os.Stderr, "Unable to write status JSON:", writeErr)
		}
		return
	}
	if options.JSON {
		out := StatusJSON(st, err, options)
		_ = json.NewEncoder(os.Stdout).Encode(out)
		return
	}
	_ = json.NewEncoder(os.Stdout).Encode(AshellStatus(st, err, options))
}
