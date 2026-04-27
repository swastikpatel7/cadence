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

// initialPlanSystemPrompt instructs Opus 4.7 to emit the PlanBlob
// structure end-to-end. Voice + structure cues come from insights.md
// §8.2.
const initialPlanSystemPrompt = `You are Cadence's plan generator. You are about to write an 8-week running plan that fits the runner's goal and current baseline. The plan must be:

1. Periodized: a build phase (W1-W3), a peak phase (W4-W6), and a taper/consolidation phase (W7-W8). Volume builds ~10% per week through the build, holds in the peak, drops 25-40% in the taper.
2. Calibrated to the user's days_per_week: that many running days per week, the rest are rest. Never schedule running on a rest day.
3. Specific: every session has a type (easy|tempo|intervals|long|recovery|race_pace), a distance in km, an intensity (easy|moderate|hard), and a one-sentence "notes_for_coach" string explaining the goal of that workout. Pace targets and duration targets are optional but encouraged when the user has given a target_pace or target_distance.

You will return JSON exactly matching the provided schema. The Load field on each session must be one of {easy, moderate, hard, peak}; map intensity → load: easy=easy, moderate=moderate, hard=hard, race_pace at peak phase=peak.

Constraints:
- The first week.start_date must equal the supplied "first_monday" date.
- Each session.date is YYYY-MM-DD and must fall on the corresponding week.
- Sum the per-week distance into total_km.
- 8 weeks total.`

// initialPlanSchema is the strict JSON-schema the model is bound to.
var initialPlanSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"weeks": map[string]any{
			"type":     "array",
			"minItems": 8,
			"maxItems": 8,
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"week_index": map[string]any{"type": "integer", "minimum": 0, "maximum": 7},
					"start_date": map[string]any{"type": "string", "format": "date"},
					"total_km":   map[string]any{"type": "number", "minimum": 0},
					"sessions": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"date":                    map[string]any{"type": "string", "format": "date"},
								"type":                    map[string]any{"type": "string", "enum": []string{"easy", "tempo", "intervals", "long", "recovery", "race_pace"}},
								"distance_km":             map[string]any{"type": "number", "minimum": 0},
								"intensity":               map[string]any{"type": "string", "enum": []string{"easy", "moderate", "hard"}},
								"pace_target_sec_per_km":  map[string]any{"type": "integer"},
								"duration_min_target":     map[string]any{"type": "integer"},
								"notes_for_coach":         map[string]any{"type": "string"},
								"load":                    map[string]any{"type": "string", "enum": []string{"easy", "moderate", "hard", "peak"}},
							},
							"required":             []string{"date", "type", "distance_km", "intensity", "load"},
							"additionalProperties": false,
						},
					},
				},
				"required":             []string{"week_index", "total_km", "sessions"},
				"additionalProperties": false,
			},
		},
	},
	"required":             []string{"weeks"},
	"additionalProperties": false,
}

// InitialPlanInput carries everything GenerateInitial needs from the
// caller. Decoupled from db rows so unit tests can construct it
// directly.
type InitialPlanInput struct {
	GoalFocus           string
	WeeklyMilesTarget   int
	DaysPerWeek         int
	TargetDistanceKM    *float64
	TargetPaceSecPerKM  *int
	RaceDate            *string
	BaselineNarrative   string
	WeeklyVolumeKMAvg   float64
	WeeklyVolumeKMP25   float64
	WeeklyVolumeKMP75   float64
	AvgPaceSecPerKM     int32
	LongestRunKM        float64
	FitnessTier         string
	FirstMonday         time.Time
	Today               time.Time
}

// GenerateInitial calls Opus 4.7 with the provided context and returns
// the parsed PlanBlob.
func GenerateInitial(
	ctx context.Context,
	client *coach.Client,
	in InitialPlanInput,
) (*PlanBlob, *coach.Result, error) {
	if !client.Available() {
		return nil, nil, coach.ErrNotConfigured
	}
	opus, _, _ := client.Models()

	userBlock := buildInitialUserPrompt(in)

	params := anthropic.MessageNewParams{
		Model:     opus,
		MaxTokens: 8192,
		System:    coach.SystemBlockWithCache(initialPlanSystemPrompt),
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userBlock)),
		},
		Thinking: coach.AdaptiveThinkingSummarized(),
		OutputConfig: anthropic.OutputConfigParam{
			Effort: anthropic.OutputConfigEffortXhigh,
			Format: anthropic.JSONOutputFormatParam{Schema: initialPlanSchema},
		},
	}

	res, err := client.Send(ctx, params)
	if err != nil {
		return nil, nil, fmt.Errorf("plan.generator_initial: send: %w", err)
	}
	parsed, err := parsePlanBlob(res.FirstText)
	if err != nil {
		// Single retry on parse failure.
		retry, rerr := client.Send(ctx, params)
		if rerr != nil {
			return nil, nil, fmt.Errorf("plan.generator_initial: retry send: %w", rerr)
		}
		parsed, err = parsePlanBlob(retry.FirstText)
		if err != nil {
			return nil, nil, fmt.Errorf("plan.generator_initial: parse failed twice: %w", err)
		}
		res = retry
	}
	if len(parsed.Weeks) != 8 {
		return nil, nil, fmt.Errorf("plan.generator_initial: expected 8 weeks, got %d", len(parsed.Weeks))
	}
	return parsed, res, nil
}

func buildInitialUserPrompt(in InitialPlanInput) string {
	var b strings.Builder
	b.WriteString("Goal:\n")
	fmt.Fprintf(&b, "- focus: %s\n", in.GoalFocus)
	fmt.Fprintf(&b, "- weekly_miles_target: %d\n", in.WeeklyMilesTarget)
	fmt.Fprintf(&b, "- days_per_week: %d\n", in.DaysPerWeek)
	if in.TargetDistanceKM != nil {
		fmt.Fprintf(&b, "- target_distance_km: %.2f\n", *in.TargetDistanceKM)
	}
	if in.TargetPaceSecPerKM != nil {
		fmt.Fprintf(&b, "- target_pace_sec_per_km: %d\n", *in.TargetPaceSecPerKM)
	}
	if in.RaceDate != nil && *in.RaceDate != "" {
		fmt.Fprintf(&b, "- race_date: %s\n", *in.RaceDate)
	}

	b.WriteString("\nBaseline:\n")
	fmt.Fprintf(&b, "- fitness_tier: %s\n", in.FitnessTier)
	fmt.Fprintf(&b, "- weekly_volume_km_avg: %.1f\n", in.WeeklyVolumeKMAvg)
	fmt.Fprintf(&b, "- weekly_volume_km_p25: %.1f\n", in.WeeklyVolumeKMP25)
	fmt.Fprintf(&b, "- weekly_volume_km_p75: %.1f\n", in.WeeklyVolumeKMP75)
	fmt.Fprintf(&b, "- avg_pace_sec_per_km: %d\n", in.AvgPaceSecPerKM)
	fmt.Fprintf(&b, "- longest_run_km: %.1f\n", in.LongestRunKM)
	if in.BaselineNarrative != "" {
		fmt.Fprintf(&b, "- narrative: %s\n", in.BaselineNarrative)
	}

	b.WriteString("\nDates:\n")
	fmt.Fprintf(&b, "- today: %s\n", in.Today.Format("2006-01-02"))
	fmt.Fprintf(&b, "- first_monday: %s\n", in.FirstMonday.Format("2006-01-02"))

	b.WriteString("\nReturn JSON matching the schema. Output 8 weeks of sessions starting from first_monday.")
	return b.String()
}

func parsePlanBlob(raw string) (*PlanBlob, error) {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	var pb PlanBlob
	if err := json.Unmarshal([]byte(s), &pb); err != nil {
		return nil, fmt.Errorf("decode planblob: %w", err)
	}
	if len(pb.Weeks) == 0 {
		return nil, fmt.Errorf("empty weeks")
	}
	return &pb, nil
}

// FirstMondayAfter returns the next Monday on or after the given day.
// Used by the handler to pin coach_plans.starts_on.
func FirstMondayAfter(t time.Time) time.Time {
	t = t.UTC()
	wd := int(t.Weekday()) // Sunday=0
	if wd == 1 {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	}
	// time.Weekday: Sun=0, Mon=1, Tue=2, ..., Sat=6
	// Days until next Monday:
	days := (1 - wd + 7) % 7
	if days == 0 {
		days = 7
	}
	out := t.AddDate(0, 0, days)
	return time.Date(out.Year(), out.Month(), out.Day(), 0, 0, 0, 0, time.UTC)
}
