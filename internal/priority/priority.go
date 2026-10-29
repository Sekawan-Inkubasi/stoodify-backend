package priority

import (
	"math"
	"strings"
	"time"
)

// Bobot sesuai PRD Stoodify §Priority Score Formula.
const (
	WeightUrgency  = 0.40
	WeightDiff     = 0.20
	WeightDuration = 0.15
	WeightImport   = 0.15
	WeightLateRisk = 0.10
)

type Input struct {
	Deadline        time.Time
	Difficulty      int // 1..5
	EstDuration     int // menit
	Type            string
	Progress        int // 0..100
	MaxDailyMinutes int
	Now             time.Time
}

func importance(taskType string) float64 {
	switch strings.ToLower(taskType) {
	case "exam", "ujian":
		return 1.0
	case "project", "proyek":
		return 0.9
	case "assignment", "tugas":
		return 0.7
	case "reading", "baca_materi":
		return 0.5
	default:
		return 0.7
	}
}

// urgency: 10 kalau deadline sudah lewat, turun linear sampai 1 pada >=14 hari.
func urgency(daysLeft float64) float64 {
	switch {
	case daysLeft <= 0:
		return 10
	case daysLeft >= 14:
		return 1
	default:
		return 10 - (daysLeft/14)*9
	}
}

type Breakdown struct {
	UrgencyScore    float64 `json:"urgency_score"`
	DifficultyScore float64 `json:"difficulty_score"`
	DurationScore   float64 `json:"duration_score"`
	ImportanceScore float64 `json:"importance_score"`
	LateRiskScore   float64 `json:"late_risk_score"`
	TotalScore      float64 `json:"total_score"`
}

// CalculateBreakdown menghitung detail bobot masing-masing faktor prioritas.
func CalculateBreakdown(in Input) Breakdown {
	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}
	daysLeft := in.Deadline.Sub(now).Hours() / 24

	maxDaily := in.MaxDailyMinutes
	if maxDaily <= 0 {
		maxDaily = 120
	}

	difficulty := float64(in.Difficulty)
	if difficulty < 1 {
		difficulty = 1
	}
	if difficulty > 5 {
		difficulty = 5
	}

	u := urgency(daysLeft) / 10
	d := (difficulty - 1) / 4
	dur := math.Min(float64(in.EstDuration)/float64(maxDaily), 1)
	imp := importance(in.Type)

	lateRisk := 0.0
	if in.Progress < 20 && daysLeft < 3 {
		lateRisk = 1
	}

	raw := u*WeightUrgency + d*WeightDiff + dur*WeightDuration + imp*WeightImport + lateRisk*WeightLateRisk
	score := raw * 100

	progress := in.Progress
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	score *= 1 - float64(progress)/100

	return Breakdown{
		UrgencyScore:    math.Round(u*WeightUrgency*100*100) / 100,
		DifficultyScore: math.Round(d*WeightDiff*100*100) / 100,
		DurationScore:   math.Round(dur*WeightDuration*100*100) / 100,
		ImportanceScore: math.Round(imp*WeightImport*100*100) / 100,
		LateRiskScore:   math.Round(lateRisk*WeightLateRisk*100*100) / 100,
		TotalScore:      math.Round(score*100) / 100,
	}
}

// Score mengembalikan 0..100. Progres mengurangi skor secara proporsional.
func Score(in Input) float64 {
	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}
	daysLeft := in.Deadline.Sub(now).Hours() / 24

	maxDaily := in.MaxDailyMinutes
	if maxDaily <= 0 {
		maxDaily = 120
	}

	difficulty := float64(in.Difficulty)
	if difficulty < 1 {
		difficulty = 1
	}
	if difficulty > 5 {
		difficulty = 5
	}

	u := urgency(daysLeft) / 10
	d := (difficulty - 1) / 4
	dur := math.Min(float64(in.EstDuration)/float64(maxDaily), 1)
	imp := importance(in.Type)

	lateRisk := 0.0
	if in.Progress < 20 && daysLeft < 3 {
		lateRisk = 1
	}

	raw := u*WeightUrgency + d*WeightDiff + dur*WeightDuration + imp*WeightImport + lateRisk*WeightLateRisk
	score := raw * 100

	progress := in.Progress
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	score *= 1 - float64(progress)/100

	return math.Round(score*100) / 100
}
