// Package campaign — variant_metrics provides A/B test comparison with statistical significance.
package campaign

import (
	"context"
	"fmt"
	"math"
)

// VariantComparisonDTO is the top-level response for variant comparison.
type VariantComparisonDTO struct {
	Variants []VariantComparisonEntry `json:"variants"`
	Tests    []SignificanceTest       `json:"tests"`
}

// VariantComparisonEntry holds per-variant metrics for comparison.
type VariantComparisonEntry struct {
	Label        string  `json:"label"`
	Sent         int     `json:"sent"`
	Delivered    int     `json:"delivered"`
	Opened       int     `json:"opened"`
	Clicked      int     `json:"clicked"`
	Captured     int     `json:"captured"`
	OpenRate     float64 `json:"open_rate"`
	ClickRate    float64 `json:"click_rate"`
	CaptureRate  float64 `json:"capture_rate"`
	DeliveryRate float64 `json:"delivery_rate"`
	IsWinner     bool    `json:"is_winner"`
}

// SignificanceTest holds a chi-squared test result between two variants.
type SignificanceTest struct {
	Metric      string  `json:"metric"`
	ChiSquared  float64 `json:"chi_squared"`
	PValue      float64 `json:"p_value"`
	Significant bool    `json:"significant"` // p < 0.05
	Winner      string  `json:"winner"`
}

// GetVariantComparison computes per-variant metrics with statistical significance.
func (s *Service) GetVariantComparison(ctx context.Context, campaignID string) (VariantComparisonDTO, error) {
	variantCounts, err := s.repo.CountEmailsByStatusAndVariant(ctx, campaignID)
	if err != nil {
		return VariantComparisonDTO{}, fmt.Errorf("campaign service: variant comparison: %w", err)
	}

	if len(variantCounts) == 0 {
		return VariantComparisonDTO{}, nil
	}

	// Build per-variant entries.
	entries := make([]VariantComparisonEntry, 0, len(variantCounts))
	for label, counts := range variantCounts {
		sent := counts["sent"] + counts["delivered"] + counts["bounced"] + counts["failed"] + counts["deferred"]
		delivered := counts["delivered"]
		opened := counts["opened"]
		clicked := counts["clicked"]
		captured := counts["captured"]

		entry := VariantComparisonEntry{
			Label:     label,
			Sent:      sent,
			Delivered: delivered,
			Opened:    opened,
			Clicked:   clicked,
			Captured:  captured,
		}
		if sent > 0 {
			entry.DeliveryRate = float64(delivered) / float64(sent) * 100
			entry.OpenRate = float64(opened) / float64(sent) * 100
			entry.ClickRate = float64(clicked) / float64(sent) * 100
			entry.CaptureRate = float64(captured) / float64(sent) * 100
		}
		entries = append(entries, entry)
	}

	// Mark winner by capture rate (primary), then click rate, then open rate.
	if len(entries) >= 2 {
		bestIdx := 0
		for i := 1; i < len(entries); i++ {
			if entries[i].CaptureRate > entries[bestIdx].CaptureRate {
				bestIdx = i
			} else if entries[i].CaptureRate == entries[bestIdx].CaptureRate && entries[i].ClickRate > entries[bestIdx].ClickRate {
				bestIdx = i
			}
		}
		entries[bestIdx].IsWinner = true
	}

	// Run chi-squared tests between pairs of variants.
	var tests []SignificanceTest
	if len(entries) == 2 {
		a, b := entries[0], entries[1]
		tests = append(tests,
			chiSquaredTest("open_rate", a.Sent, a.Opened, b.Sent, b.Opened, a.Label, b.Label),
			chiSquaredTest("click_rate", a.Sent, a.Clicked, b.Sent, b.Clicked, a.Label, b.Label),
			chiSquaredTest("capture_rate", a.Sent, a.Captured, b.Sent, b.Captured, a.Label, b.Label),
		)
	}

	return VariantComparisonDTO{Variants: entries, Tests: tests}, nil
}

// chiSquaredTest performs a 2x2 chi-squared test for two proportions.
func chiSquaredTest(metric string, n1, s1, n2, s2 int, label1, label2 string) SignificanceTest {
	result := SignificanceTest{Metric: metric}

	total := n1 + n2
	totalSuccess := s1 + s2
	if total == 0 || totalSuccess == 0 || totalSuccess == total {
		return result
	}

	// Expected values under null hypothesis (pooled proportion).
	p := float64(totalSuccess) / float64(total)
	e11 := p * float64(n1)
	e12 := (1 - p) * float64(n1)
	e21 := p * float64(n2)
	e22 := (1 - p) * float64(n2)

	// Chi-squared statistic with Yates' correction.
	chi2 := yatesTerm(float64(s1), e11) +
		yatesTerm(float64(n1-s1), e12) +
		yatesTerm(float64(s2), e21) +
		yatesTerm(float64(n2-s2), e22)

	result.ChiSquared = math.Round(chi2*1000) / 1000
	result.PValue = math.Round(chiSquaredPValue(chi2, 1)*10000) / 10000
	result.Significant = result.PValue < 0.05

	// Determine winner.
	r1 := float64(s1) / float64(n1)
	r2 := float64(s2) / float64(n2)
	if r1 > r2 {
		result.Winner = label1
	} else if r2 > r1 {
		result.Winner = label2
	}

	return result
}

func yatesTerm(observed, expected float64) float64 {
	if expected == 0 {
		return 0
	}
	diff := math.Abs(observed-expected) - 0.5
	if diff < 0 {
		diff = 0
	}
	return diff * diff / expected
}

// chiSquaredPValue approximates the p-value for chi-squared with 1 degree of freedom
// using the complementary error function.
func chiSquaredPValue(chi2 float64, _ int) float64 {
	if chi2 <= 0 {
		return 1.0
	}
	// For 1 df: p = erfc(sqrt(chi2/2))
	return math.Erfc(math.Sqrt(chi2 / 2))
}
