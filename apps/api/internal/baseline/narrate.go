package baseline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/swastikpatel7/cadence/apps/api/internal/coach"
)

// NarrativeOutput is the parsed JSON payload Opus 4.7 returns. Shape
// is enforced by an in-prompt JSON example + post-parse validation in
// parseNarrative; Anthropic's `output_config.format` is intentionally
// not used (it rejects half the constraints we care about — see
// docs/CHANGELOG.md 2026-04-30).
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

const baselineSystemPrompt = `You are Cadence's onboarding coach. Your job is to read a runner's last 30-90 days of running data and write ONE short calibration paragraph that gives them a felt sense of where they are.

Voice: warm, observational, specific, never preachy. 2-4 sentences, ≤ 90 words. No exclamation marks, no second-person commands. Reference at least one specific stat the user can verify (a date, a pace, a distance). End with a one-clause hint at where Cadence will take them next month.

Output: respond with ONE JSON object and nothing else — no markdown fence, no commentary. Shape:

{
  "fitness_tier": "T1" | "T2" | "T3" | "T4" | "T5",
  "narrative": "string",
  "consistency_score": 0..100,
  "avg_pace_at_distance": { "5": int|null, "10": int|null, "21.1": int|null, "42.2": int|null }
}

The narrative goes in the "narrative" field. Choose fitness_tier from {T1, T2, T3, T4, T5}: T1 (sedentary/returning), T2 (foundation, ≤25km/wk), T3 (consistent, 25-45km/wk), T4 (advanced, 45-70km/wk), T5 (competitive, ≥70km/wk). consistency_score is an integer between 0 and 100 inclusive (0 = no recent activity, 100 = ran on every available day). avg_pace_at_distance values are seconds-per-km integers, or null when there is no evidence for that distance.`

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
	switch out.FitnessTier {
	case "T1", "T2", "T3", "T4", "T5":
	case "":
		return nil, fmt.Errorf("empty fitness_tier")
	default:
		return nil, fmt.Errorf("fitness_tier %q not in {T1..T5}", out.FitnessTier)
	}
	// Clamp consistency_score post-parse — schema can't express the bound,
	// the prompt asks for 0..100; if the model strays we clip rather than
	// fail the whole onboarding (the user-visible narrative is fine).
	if out.ConsistencyScore < 0 {
		out.ConsistencyScore = 0
	} else if out.ConsistencyScore > 100 {
		out.ConsistencyScore = 100
	}
	return &out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
