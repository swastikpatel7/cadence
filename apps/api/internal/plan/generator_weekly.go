package plan

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/swastikpatel7/cadence/apps/api/internal/coach"
)

// Shape is enforced by the in-prompt JSON example below plus post-parse
// validation in parsePlanBlob; we deliberately do not use Anthropic's
// `output_config.format` (rejects half the constraints we care about —
// see docs/CHANGELOG.md 2026-04-30).
const weeklyRefreshSystemPrompt = `You are Cadence's weekly plan adjuster. You will receive (a) the runner's current 8-week plan, (b) what they actually completed last week, and (c) the date the next week starts on.

Your job: produce ONE week of sessions (week_index always 0, start_date = next_week_start) that smoothly continues the periodized arc. Adjust based on completion:

- If they hit ≥ 90% of last week's prescribed volume: progress as planned (next step in the build).
- If they hit 60-90%: hold volume, swap one hard day for an easy day if recovery looks tight.
- If they hit < 60% or skipped 2+ days: pull back 15-25% in volume, ease intensity for 3+ days.

Stay in voice with the original plan: same session-type vocabulary, same intensity ladder.

Output: respond with ONE JSON object and nothing else — no markdown fence, no commentary. Shape:

{
  "weeks": [
    {
      "week_index": 0,
      "start_date": "YYYY-MM-DD",
      "total_km": number,
      "sessions": [
        {
          "date": "YYYY-MM-DD",
          "type": "easy" | "tempo" | "intervals" | "long" | "recovery" | "race_pace",
          "distance_km": number,
          "intensity": "easy" | "moderate" | "hard",
          "pace_target_sec_per_km": int,        // optional
          "duration_min_target": int,           // optional
          "notes_for_coach": "string",          // optional
          "load": "easy" | "moderate" | "hard" | "peak"
        }
      ]
    }
  ]
}

Emit exactly one week object with week_index = 0. Do not include any field not listed above.`

// WeeklyRefreshInput is the structured prior-week summary the worker
// composes and hands to GenerateWeekly.
type WeeklyRefreshInput struct {
	NextWeekStart       time.Time
	GoalFocus           string
	WeeklyMilesTarget   int
	DaysPerWeek         int
	BaselineFitnessTier string
	BaselineNarrative   string
	PriorWeekPrescribed []SessionRecap
	PriorWeekActual     []ActivityRecap
	Reason              string
}

// SessionRecap is one row in the "what we asked for last week" section.
type SessionRecap struct {
	Date       string
	Type       string
	DistanceKM float64
	Intensity  string
}

// ActivityRecap is one row in the "what they actually did" section.
type ActivityRecap struct {
	Date            string
	DistanceKM      float64
	AvgPaceSecPerKM int
	DurationSec     int
}

// GenerateWeekly calls Sonnet 4.6 to produce the next week's plan.
func GenerateWeekly(
	ctx context.Context,
	client *coach.Client,
	in WeeklyRefreshInput,
) (*PlanBlob, *coach.Result, error) {
	if !client.Available() {
		return nil, nil, coach.ErrNotConfigured
	}
	_, sonnet, _ := client.Models()

	user := buildWeeklyUserPrompt(in)
	params := anthropic.MessageNewParams{
		Model:     sonnet,
		MaxTokens: 3072,
		System:    coach.SystemBlockWithCache(weeklyRefreshSystemPrompt),
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(user)),
		},
		Thinking: coach.AdaptiveThinkingOmitted(),
		OutputConfig: anthropic.OutputConfigParam{
			Effort: anthropic.OutputConfigEffortMedium,
		},
	}

	res, err := client.Send(ctx, params)
	if err != nil {
		return nil, nil, fmt.Errorf("plan.generator_weekly: send: %w", err)
	}
	parsed, err := parsePlanBlob(res.FirstText)
	if err != nil {
		retry, rerr := client.Send(ctx, params)
		if rerr != nil {
			return nil, nil, fmt.Errorf("plan.generator_weekly: retry send: %w", rerr)
		}
		parsed, err = parsePlanBlob(retry.FirstText)
		if err != nil {
			return nil, nil, fmt.Errorf("plan.generator_weekly: parse failed twice: %w", err)
		}
		res = retry
	}
	if len(parsed.Weeks) != 1 {
		return nil, nil, fmt.Errorf("plan.generator_weekly: expected 1 week, got %d", len(parsed.Weeks))
	}
	// Schema can't pin week_index to 0 anymore (min=max=0 was rejected);
	// re-assert here so a model misfire can't slip a non-zero index past
	// callers that assume single-week refresh shape.
	if parsed.Weeks[0].WeekIndex != 0 {
		return nil, nil, fmt.Errorf("plan.generator_weekly: expected week_index 0, got %d", parsed.Weeks[0].WeekIndex)
	}
	return parsed, res, nil
}

// micro-summary plumbing kept here too so the Haiku call sits next to
// its siblings.

const microSummarySystemPrompt = `You are Cadence's session reviewer. Output ONE sentence (≤ 24 words) that compares what the runner actually ran with what was prescribed. Reference at least one number (pace, distance, or HR). Voice: warm, observational, no exclamation marks. Plain text only — no markdown, no quotes.`

// MicroSummaryInput holds the Haiku 4.5 inputs.
type MicroSummaryInput struct {
	Prescribed PrescribedSessionArgs
	ActualKM   float64
	ActualPace int
	ActualDur  int
	ActualHR   *int32
	StartedAt  time.Time
}

// GenerateMicroSummary calls Haiku 4.5 (no thinking, no schema, plain
// text). Returns the plain text body.
func GenerateMicroSummary(
	ctx context.Context,
	client *coach.Client,
	in MicroSummaryInput,
) (string, *coach.Result, error) {
	if !client.Available() {
		return "", nil, coach.ErrNotConfigured
	}
	_, _, haiku := client.Models()

	user := buildMicroSummaryPrompt(in)
	params := anthropic.MessageNewParams{
		Model:     haiku,
		MaxTokens: 200,
		System:    coach.SystemBlockWithCache(microSummarySystemPrompt),
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(user)),
		},
	}
	res, err := client.Send(ctx, params)
	if err != nil {
		return "", nil, fmt.Errorf("plan.generator_micro: send: %w", err)
	}
	body := strings.TrimSpace(res.FirstText)
	return body, res, nil
}

func buildWeeklyUserPrompt(in WeeklyRefreshInput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Goal: focus=%s, weekly_miles_target=%d, days_per_week=%d.\n",
		in.GoalFocus, in.WeeklyMilesTarget, in.DaysPerWeek)
	fmt.Fprintf(&b, "Baseline tier: %s.\n", in.BaselineFitnessTier)
	if in.BaselineNarrative != "" {
		fmt.Fprintf(&b, "Baseline narrative: %s\n", in.BaselineNarrative)
	}
	fmt.Fprintf(&b, "Refresh reason: %s.\n", in.Reason)
	fmt.Fprintf(&b, "Next week start (Monday): %s.\n\n", in.NextWeekStart.Format("2006-01-02"))

	b.WriteString("Last week prescribed:\n")
	if len(in.PriorWeekPrescribed) == 0 {
		b.WriteString("- (no prescribed sessions; first refresh after onboarding gap)\n")
	}
	for _, s := range in.PriorWeekPrescribed {
		fmt.Fprintf(&b, "- %s: %s, %.1f km, intensity=%s\n", s.Date, s.Type, s.DistanceKM, s.Intensity)
	}

	b.WriteString("\nLast week actual runs:\n")
	if len(in.PriorWeekActual) == 0 {
		b.WriteString("- (no completed runs)\n")
	}
	for _, a := range in.PriorWeekActual {
		fmt.Fprintf(&b, "- %s: %.1f km, %ds/km avg, %ds total\n", a.Date, a.DistanceKM, a.AvgPaceSecPerKM, a.DurationSec)
	}

	b.WriteString("\nReturn the JSON week per the schema.")
	return b.String()
}

func buildMicroSummaryPrompt(in MicroSummaryInput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Prescribed (%s): %s, %.1f km, intensity %s.",
		in.Prescribed.Date, in.Prescribed.Type, in.Prescribed.DistanceKM, in.Prescribed.Intensity)
	if in.Prescribed.PaceTargetSecPerKM != nil {
		fmt.Fprintf(&b, " Pace target %ds/km.", *in.Prescribed.PaceTargetSecPerKM)
	}
	if in.Prescribed.NotesForCoach != "" {
		fmt.Fprintf(&b, " Coach note: %s", in.Prescribed.NotesForCoach)
	}
	fmt.Fprintf(&b, "\nActual: %.1f km in %ds (%ds/km avg)",
		in.ActualKM, in.ActualDur, in.ActualPace)
	if in.ActualHR != nil {
		fmt.Fprintf(&b, ", avg HR %d", *in.ActualHR)
	}
	b.WriteString(".\nWrite the one-sentence review.")
	return b.String()
}

// MarshalPlan is a tiny helper used by the workers when they need to
// turn a PlanBlob back into the bytes for the `plan jsonb` column.
func MarshalPlan(pb *PlanBlob) ([]byte, error) {
	return json.Marshal(pb)
}
