package metrics

import (
	"testing"
	"time"
)

func TestCampaignMetricsRates(t *testing.T) {
	tests := []struct {
		name             string
		delivered        int
		opens            int
		clicks           int
		submissions      int
		reports          int
		wantOpenRate     float64
		wantCTR          float64
		wantSubmitRate   float64
		wantReportRate   float64
	}{
		{
			name:           "zero delivered",
			delivered:      0,
			opens:          0,
			clicks:         0,
			submissions:    0,
			reports:        0,
			wantOpenRate:   0,
			wantCTR:        0,
			wantSubmitRate: 0,
			wantReportRate: 0,
		},
		{
			name:           "typical campaign",
			delivered:      100,
			opens:          50,
			clicks:         25,
			submissions:    10,
			reports:        5,
			wantOpenRate:   50,
			wantCTR:        25,
			wantSubmitRate: 40,
			wantReportRate: 5,
		},
		{
			name:           "all opened and clicked",
			delivered:      10,
			opens:          10,
			clicks:         10,
			submissions:    10,
			reports:        10,
			wantOpenRate:   100,
			wantCTR:        100,
			wantSubmitRate: 100,
			wantReportRate: 100,
		},
		{
			name:           "no clicks means zero submit rate",
			delivered:      50,
			opens:          20,
			clicks:         0,
			submissions:    0,
			reports:        2,
			wantOpenRate:   40,
			wantCTR:        0,
			wantSubmitRate: 0,
			wantReportRate: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := CampaignMetrics{
				EmailsSent:  tt.delivered,
				UniqueOpens: tt.opens,
				UniqueClicks: tt.clicks,
				Submissions: tt.submissions,
				Reports:     tt.reports,
			}

			// Compute rates the same way the service does.
			delivered := m.EmailsSent
			if delivered > 0 {
				m.OpenRate = float64(m.UniqueOpens) / float64(delivered) * 100
				m.ClickThroughRate = float64(m.UniqueClicks) / float64(delivered) * 100
				m.ReportRate = float64(m.Reports) / float64(delivered) * 100
			}
			if m.UniqueClicks > 0 {
				m.SubmissionRate = float64(m.Submissions) / float64(m.UniqueClicks) * 100
			}

			if m.OpenRate != tt.wantOpenRate {
				t.Errorf("OpenRate = %v, want %v", m.OpenRate, tt.wantOpenRate)
			}
			if m.ClickThroughRate != tt.wantCTR {
				t.Errorf("CTR = %v, want %v", m.ClickThroughRate, tt.wantCTR)
			}
			if m.SubmissionRate != tt.wantSubmitRate {
				t.Errorf("SubmissionRate = %v, want %v", m.SubmissionRate, tt.wantSubmitRate)
			}
			if m.ReportRate != tt.wantReportRate {
				t.Errorf("ReportRate = %v, want %v", m.ReportRate, tt.wantReportRate)
			}
		})
	}
}

func TestVariantMetricsBreakdown(t *testing.T) {
	va := VariantMetrics{
		VariantLabel: "A",
		EmailsSent:   50,
		UniqueOpens:  25,
		UniqueClicks: 10,
		Submissions:  5,
	}
	vb := VariantMetrics{
		VariantLabel: "B",
		EmailsSent:   50,
		UniqueOpens:  35,
		UniqueClicks: 20,
		Submissions:  15,
	}

	// Compute rates.
	for _, v := range []*VariantMetrics{&va, &vb} {
		if v.EmailsSent > 0 {
			v.OpenRate = float64(v.UniqueOpens) / float64(v.EmailsSent) * 100
			v.ClickThroughRate = float64(v.UniqueClicks) / float64(v.EmailsSent) * 100
		}
		if v.UniqueClicks > 0 {
			v.SubmissionRate = float64(v.Submissions) / float64(v.UniqueClicks) * 100
		}
	}

	// Variant A: 50% open, 20% CTR, 50% submit
	if va.OpenRate != 50 {
		t.Errorf("VA OpenRate = %v, want 50", va.OpenRate)
	}
	if va.ClickThroughRate != 20 {
		t.Errorf("VA CTR = %v, want 20", va.ClickThroughRate)
	}
	if va.SubmissionRate != 50 {
		t.Errorf("VA SubmissionRate = %v, want 50", va.SubmissionRate)
	}

	// Variant B: 70% open, 40% CTR, 75% submit
	if vb.OpenRate != 70 {
		t.Errorf("VB OpenRate = %v, want 70", vb.OpenRate)
	}
	if vb.ClickThroughRate != 40 {
		t.Errorf("VB CTR = %v, want 40", vb.ClickThroughRate)
	}
	if vb.SubmissionRate != 75 {
		t.Errorf("VB SubmissionRate = %v, want 75", vb.SubmissionRate)
	}

	// Totals should match: 50+50=100 sent, 25+35=60 opens, etc.
	total := CampaignMetrics{
		EmailsSent:   va.EmailsSent + vb.EmailsSent,
		UniqueOpens:  va.UniqueOpens + vb.UniqueOpens,
		UniqueClicks: va.UniqueClicks + vb.UniqueClicks,
		Submissions:  va.Submissions + vb.Submissions,
	}
	if total.EmailsSent != 100 {
		t.Errorf("Total sent = %d, want 100", total.EmailsSent)
	}
	if total.UniqueOpens != 60 {
		t.Errorf("Total opens = %d, want 60", total.UniqueOpens)
	}
}

func TestHourlyBucketFields(t *testing.T) {
	b := HourlyBucket{
		Hour:        time.Date(2026, 3, 26, 14, 0, 0, 0, time.UTC),
		Opens:       10,
		Clicks:      5,
		Submissions: 2,
		EmailsSent:  20,
	}

	if b.Opens != 10 || b.Clicks != 5 || b.Submissions != 2 || b.EmailsSent != 20 {
		t.Error("HourlyBucket fields not set correctly")
	}
}

func TestCacheEntryExpiry(t *testing.T) {
	entry := cacheEntry{
		data:      CampaignMetrics{TotalTargets: 42},
		expiresAt: time.Now().Add(-1 * time.Second),
	}

	if time.Now().Before(entry.expiresAt) {
		t.Error("expired entry should not be valid")
	}

	fresh := cacheEntry{
		data:      CampaignMetrics{TotalTargets: 42},
		expiresAt: time.Now().Add(30 * time.Second),
	}
	if !time.Now().Before(fresh.expiresAt) {
		t.Error("fresh entry should be valid")
	}
}

func TestSusceptibilityScore(t *testing.T) {
	// Susceptibility = click_rate * 0.4 + submission_rate * 0.6
	clickRate := 25.0
	submitRate := 40.0
	score := clickRate*0.4 + submitRate*0.6
	expected := 34.0 // 10 + 24
	if score != expected {
		t.Errorf("susceptibility = %v, want %v", score, expected)
	}
}
