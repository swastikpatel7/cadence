package baseline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/swastikpatel7/cadence/apps/api/internal/coach"
)

// NarrativeOutput is the parsed JSON payload Opus 4.7 returns. Schema
// pinned via OutputConfig.Format JSON-schema (insights.md §8.1).
//
// The narrative field is the only piece exposed on the wire; the rest
// echoes structure we already have so the model can reason about it
// while it composes the line.
type NarrativeOutput struct {
	FitnessTier      string         `json:"fitness_tier"`
	Narrative        string         `json:"narrative"`
	ConsistencyScore int            `json:"consistency_score"`
	AvgPaceAtDistance map[string]int `json:"avg_pace_at_distance,omitempty"`
}

// narrativeJSONSchema is the OpenAI/Anthropic-compatible JSON-schema
// the model is held to. Keep this in sync with NarrativeOutput.
var narrativeJSONSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"fitness_tier": map[string]any{
			"type": "string",
			"enum": []string{"T1", "T2", "T3", "T4", "T5"},
		},
		"narrative": map[string]any{
			"type":      "string",
			"minLength": 60,
			"maxLength": 600,
		},
		"consistency_score": map[string]any{
			"type":    "integer",
			"minimum": 0,
			"maximum": 100,
		},
		"avg_pace_at_distance": map[string]any{
			"type":                 "object",
			"additionalProperties": map[string]any{"type": "integer"},
		},
	},
	"required":             []string{"fitness_tier", "narrative", "consistency_score"},
	"additionalProperties": false,
}

const baselineSystemPrompt = `You are Cadence's onboarding coach. Your job is to read a runner's last 30-90 days of running data and write ONE short calibration paragraph that gives them a felt sense of where they are.

Voice: warm, observational, specific, never preachy. 2-4 sentences, ≤ 90 words. No exclamation marks, no second-person commands. Reference at least one specific stat the user can verify (a date, a pace, a distance). End with a one-clause hint at where Cadence will take them next month.

You will return JSON exactly matching the provided schema. The narrative goes in the "narrative" field. Choose fitness_tier from {T1, T2, T3, T4, T5}: T1 (sedentary/returning), T2 (foundation, ≤25km/wk), T3 (consistent, 25-45km/wk), T4 (advanced, 45-70km/wk), T5 (competitive, ≥70km/wk).`

// Narrate calls Opus 4.7 with the volume curve + recent runs context
// and returns the parsed JSON. Returns the *coach.Result alongside so
// the caller can persist token counters.
func Narrate(
	ctx context.Context,
	client *coach.Client,
	num *Numeric,
) (*NarrativeOutput, *coach.Result, error) {
	if !client.Available() {
		return nil, nil, coach.ErrNotConfigured
	}

	userBlock := buildUserPrompt(num)
	opus, _, _ := client.Models()

	params := anthropic.MessageNewParams{
		Model:     opus,
		MaxTokens: 4096,
		System:    coach.SystemBlockWithCache(baselineSystemPrompt),
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userBlock)),
		},
		Thinking: coach.AdaptiveThinkingSummarized(),
		OutputConfig: anthropic.OutputConfigParam{
			Effort: anthropic.OutputConfigEffortXhigh,
			Format: anthropic.JSONOutputFormatParam{Schema: narrativeJSONSchema},
		},
	}

	res, err := client.Send(ctx, params)
	if err != nil {
		return nil, nil, fmt.Errorf("baseline.narrate: send: %w", err)
	}

	parsed, err := parseNarrative(res.FirstText)
	if err != nil {
		// Single retry on parse error per api.md §4.1 stage 3 ("retry once
		// with the same prompt; if it fails twice, surface as error").
		retryRes, retryErr := client.Send(ctx, params)
		if retryErr != nil {
			return nil, nil, fmt.Errorf("baseline.narrate: retry send: %w", retryErr)
		}
		parsed, err = parseNarrative(retryRes.FirstText)
		if err != nil {
			return nil, nil, fmt.Errorf("baseline.narrate: parse failed twice: %w", err)
		}
		res = retryRes
	}
	return parsed, res, nil
}

func buildUserPrompt(num *Numeric) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Window: last %d days. Activity count: %d.\n", num.WindowDays, num.ActivityCount)
	fmt.Fprintf(&b, "Weekly volume (km): avg=%.1f, p25=%.1f, p75=%.1f. Long run: %.1f km. Average pace: %ds/km.\n",
		num.WeeklyVolumeKMAvg, num.WeeklyVolumeKMP25, num.WeeklyVolumeKMP75, num.LongestRunKM, num.AvgPaceSecPerKM)
	if len(num.WeeklyVolumesKM) > 0 {
		b.WriteString("Per-week volumes (km, oldest → newest): ")
		for i, w := range num.WeeklyVolumesKM {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%.1f", w)
		}
		b.WriteString("\n")
	}
	if len(num.AvgPaceAtDistance) > 0 {
		b.WriteString("Best pace observed at distance: ")
		first := true
		for d, p := range num.AvgPaceAtDistance {
			if !first {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%dK=%ds/km", d, p)
			first = false
		}
		b.WriteString("\n")
	}
	if len(num.RecentRunSummaries) > 0 {
		b.WriteString("Five most recent runs:\n")
		for _, r := range num.RecentRunSummaries {
			fmt.Fprintf(&b, "- %s: %.1f km in %ds (%ds/km)",
				r.StartTime.Format("Jan 2"), r.DistanceKM, r.DurationSec, r.AvgPaceSecKM)
			if r.AvgHR != nil {
				fmt.Fprintf(&b, ", avg HR %d", *r.AvgHR)
			}
			b.WriteString("\n")
		}
	}
	b.WriteString("\nRespond with the JSON described in the system prompt schema.")
	return b.String()
}

// parseNarrative attempts to JSON-decode the model output, tolerating
// fenced code blocks and leading whitespace that real models still
// emit even with strict output_config.
func parseNarrative(raw string) (*NarrativeOutput, error) {
	s := strings.TrimSpace(raw)
	// Strip ```json ... ``` fence if present.
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	var out NarrativeOutput
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, fmt.Errorf("decode: %w (raw=%q)", err, truncate(raw, 240))
	}
	if out.Narrative == "" {
		return nil, fmt.Errorf("empty narrative")
	}
	if out.FitnessTier == "" {
		return nil, fmt.Errorf("empty fitness_tier")
	}
	return &out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
