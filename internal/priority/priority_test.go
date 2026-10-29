package priority

import (
	"testing"
	"time"
)

func TestCalculateBreakdown(t *testing.T) {
	now := time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		input   Input
		wantMin float64
		wantMax float64
	}{
		{
			name: "Tugas mendesak dan sulit (deadline besok)",
			input: Input{
				Deadline:        now.Add(24 * time.Hour),
				Difficulty:      5,
				EstDuration:     120,
				Type:            "exam",
				Progress:        0,
				MaxDailyMinutes: 180,
				Now:             now,
			},
			wantMin: 70.0,
			wantMax: 100.0,
		},
		{
			name: "Tugas santai jauh hari (deadline 20 hari lagi)",
			input: Input{
				Deadline:        now.Add(20 * 24 * time.Hour),
				Difficulty:      1,
				EstDuration:     30,
				Type:            "reading",
				Progress:        50,
				MaxDailyMinutes: 180,
				Now:             now,
			},
			wantMin: 5.0,
			wantMax: 50.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bd := CalculateBreakdown(tc.input)
			if bd.TotalScore < tc.wantMin || bd.TotalScore > tc.wantMax {
				t.Errorf("expected total_score between %v and %v, got %v", tc.wantMin, tc.wantMax, bd.TotalScore)
			}
			if bd.UrgencyScore <= 0 || bd.DifficultyScore < 0 {
				t.Errorf("sub-scores should be non-negative, got %+v", bd)
			}
		})
	}
}

func TestScoreFormula(t *testing.T) {
	now := time.Now()
	scoreHigh := Score(Input{
		Deadline:        now.Add(2 * time.Hour),
		Difficulty:      5,
		EstDuration:     120,
		Type:            "exam",
		Progress:        0,
		MaxDailyMinutes: 120,
		Now:             now,
	})
	if scoreHigh < 70 {
		t.Errorf("expected high score > 70, got: %f", scoreHigh)
	}

	scoreDone := Score(Input{
		Deadline:        now.Add(2 * time.Hour),
		Difficulty:      5,
		EstDuration:     120,
		Type:            "exam",
		Progress:        100,
		MaxDailyMinutes: 120,
		Now:             now,
	})
	if scoreDone != 0 {
		t.Errorf("expected progress 100 to yield 0 score, got: %f", scoreDone)
	}
}
