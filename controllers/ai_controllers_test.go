package controllers

import (
	"encoding/json"
	"testing"
)

func TestParseSchedulerResponseBentukRespons(t *testing.T) {
	slot := `{"task_id":7,"start_time":"16:30","duration":60,"target_progress":30,"priority":"high","reason":"deadline dekat"}`

	cases := []struct {
		name     string
		raw      string
		wantLen  int
		wantDate string
		wantEnd  string
	}{
		{name: "sessions internal", raw: `{"sessions":[` + slot + `]}`, wantLen: 1, wantEnd: "17:30"},
		{name: "kontrak session_date", raw: `{"data":[{"task_id":7,"session_date":"2026-11-14","start_time":"16:30","end_time":"17:30","duration":60}]}`, wantLen: 1, wantDate: "2026-11-14", wantEnd: "17:30"},
		{name: "array polos", raw: `[` + slot + `]`, wantLen: 1, wantEnd: "17:30"},
		{name: "sessions menang atas data", raw: `{"sessions":[` + slot + `],"data":[` + slot + `,` + slot + `]}`, wantLen: 1, wantEnd: "17:30"},
		{name: "tidak ada slot", raw: `{"sessions":[]}`, wantLen: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			slots := parseSchedulerResponse(json.RawMessage(tc.raw))
			if len(slots) != tc.wantLen {
				t.Fatalf("jumlah slot = %d, mau %d", len(slots), tc.wantLen)
			}
			if tc.wantLen == 0 {
				return
			}
			if slots[0].TaskID != 7 {
				t.Errorf("task_id = %d, mau 7", slots[0].TaskID)
			}
			if slots[0].RecommendedDate != tc.wantDate {
				t.Errorf("tanggal = %q, mau %q", slots[0].RecommendedDate, tc.wantDate)
			}
			if slots[0].EndTime != tc.wantEnd {
				t.Errorf("end_time = %q, mau %q", slots[0].EndTime, tc.wantEnd)
			}
		})
	}
}

func TestParsePredictorResponseBentukRespons(t *testing.T) {
	kontrak := `{"predicted_topic":"Integral Tentu","confidence_score":0.88,"summary":"ringkasan","prerequisites":["Turunan Dasar","Integral Substitusi"],"key_concepts":["Teorema Dasar"],"suggested_activities":["Latihan A","Latihan B"]}`
	internal := `{"predicted_topic":"Limit Fungsi","confidence":72,"reason":"karena X","prerequisites":"Aljabar, Fungsi","preparation_suggestion":"Baca bab 3"}`

	t.Run("gaya kontrak dinormalisasi", func(t *testing.T) {
		hasil, ok := parsePredictorResponse(json.RawMessage(kontrak))
		if !ok {
			t.Fatal("respons kontrak ditolak")
		}
		if hasil.PredictedMaterial != "Integral Tentu" {
			t.Errorf("predicted = %q", hasil.PredictedMaterial)
		}
		if hasil.Confidence != 88 {
			t.Errorf("confidence = %v, mau 88 (0.88 dikali 100)", hasil.Confidence)
		}
		if hasil.Reason != "ringkasan" {
			t.Errorf("reason = %q, mau dari summary", hasil.Reason)
		}
		if hasil.Prerequisites != "Turunan Dasar, Integral Substitusi" {
			t.Errorf("prerequisites = %q", hasil.Prerequisites)
		}
		if hasil.Preparation != "Latihan A; Latihan B" {
			t.Errorf("preparation = %q, mau dari suggested_activities", hasil.Preparation)
		}
	})

	t.Run("dibungkus data", func(t *testing.T) {
		hasil, ok := parsePredictorResponse(json.RawMessage(`{"data":` + internal + `}`))
		if !ok {
			t.Fatal("respons ber-wrapper data ditolak")
		}
		if hasil.Confidence != 72 || hasil.Reason != "karena X" || hasil.Prerequisites != "Aljabar, Fungsi" {
			t.Errorf("normalisasi salah: %+v", hasil)
		}
	})

	t.Run("tanpa topik dianggap gagal", func(t *testing.T) {
		if _, ok := parsePredictorResponse(json.RawMessage(`{"confidence":90}`)); ok {
			t.Error("respons tanpa predicted_topic seharusnya gagal agar fallback jalan")
		}
	})
}

func TestAiTimeoutDefault(t *testing.T) {
	t.Setenv("AI_TIMEOUT_SECONDS", "")
	if got := aiTimeout().Seconds(); got != 15 {
		t.Errorf("timeout default = %v detik, mau 15", got)
	}
	t.Setenv("AI_TIMEOUT_SECONDS", "5")
	if got := aiTimeout().Seconds(); got != 5 {
		t.Errorf("timeout env = %v detik, mau 5", got)
	}
	t.Setenv("AI_TIMEOUT_SECONDS", "bukan-angka")
	if got := aiTimeout().Seconds(); got != 15 {
		t.Errorf("timeout invalid = %v detik, mau 15", got)
	}
}
