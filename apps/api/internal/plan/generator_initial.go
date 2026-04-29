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
// §8.2. Shape is enforced by the in-prompt JSON example below plus
// post-parse validation in parsePlanBlob; we deliberately do not use
// Anthropic's `output_config.format` (it rejects `minItems > 1`,
// `minimum`/`maximum`, and string-length bounds — see
// docs/CHANGELOG.md 2026-04-30).
const initialPlanSystemPrompt = `You are Cadence's plan generator. You are about to write an 8-week running plan that fits the runner's goal and current baseline. The plan must be:

1. Periodized: a build phase (W1-W3), a peak phase (W4-W6), and a taper/consolidation phase (W7-W8). Volume builds ~10% per week through the build, holds in the peak, drops 25-40% in the taper.
2. Calibrated to the user's days_per_week: that many running days per week, the rest are rest. Never schedule running on a rest day.
3. Specific: every session has a type (easy|tempo|intervals|long|recovery|race_pace), a distance in km, an intensity (easy|moderate|hard), and a one-sentence "notes_for_coach" string explaining the goal of that workout. Pace targets and duration targets are optional but encouraged when the user has given a target_pace or target_distance.

The Load field on each session must be one of {easy, moderate, hard, peak}; map intensity → load: easy=easy, moderate=moderate, hard=hard, race_pace at peak phase=peak.

Constraints:
- The first week.start_date must equal the supplied "first_monday" date.
- Each session.date is YYYY-MM-DD and must fall on the corresponding week.
- Sum the per-week distance into total_km.
- 8 weeks total. week_index runs 0..7 inclusive (week 0 is first_monday).
- All distances are non-negative kilometers.

Output: respond with ONE JSON object and nothing else — no markdown fence, no commentary. Shape:

{
  "weeks": [
    {
      "week_index": 0..7,
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

Emit exactly 8 week objects. Do not include any field not listed above.`

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
	// All bound and enum checks live here — we deliberately don't use
	// `output_config.format` (rejects half the constraints we care about),
	// so this is the only enforcement layer. Parse-twice retry in the
	// generator covers transient strays.
	for i, w := range pb.Weeks {
		if w.WeekIndex < 0 || w.WeekIndex > 7 {
			return nil, fmt.Errorf("week %d: week_index %d out of range 0..7", i, w.WeekIndex)
		}
		if w.TotalKM < 0 {
			return nil, fmt.Errorf("week %d: total_km %f is negative", i, w.TotalKM)
		}
		for j, sess := range w.Sessions {
			if sess.DistanceKM < 0 {
				return nil, fmt.Errorf("week %d session %d: distance_km %f is negative", i, j, sess.DistanceKM)
			}
			if !validSessionType(sess.Type) {
				return nil, fmt.Errorf("week %d session %d: type %q not in {easy,tempo,intervals,long,recovery,race_pace}", i, j, sess.Type)
			}
			if !validIntensity(sess.Intensity) {
				return nil, fmt.Errorf("week %d session %d: intensity %q not in {easy,moderate,hard}", i, j, sess.Intensity)
			}
			if !validLoad(sess.Load) {
				return nil, fmt.Errorf("week %d session %d: load %q not in {easy,moderate,hard,peak}", i, j, sess.Load)
			}
		}
	}
	return &pb, nil
}

func validSessionType(s string) bool {
	switch s {
	case "easy", "tempo", "intervals", "long", "recovery", "race_pace":
		return true
	}
	return false
}

func validIntensity(s string) bool {
	switch s {
	case "easy", "moderate", "hard":
		return true
	}
	return false
}

func validLoad(s string) bool {
	switch s {
	case "easy", "moderate", "hard", "peak":
		return true
	}
	return false
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
