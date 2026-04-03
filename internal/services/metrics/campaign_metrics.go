// Package metrics provides campaign metrics aggregation.
package metrics

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// CampaignMetrics holds computed metrics for a campaign.
type CampaignMetrics struct {
	CampaignID       string           `json:"campaign_id"`
	TotalTargets     int              `json:"total_targets"`
	EmailsSent       int              `json:"emails_sent"`
	EmailsDelivered  int              `json:"emails_delivered"`
	EmailsBounced    int              `json:"emails_bounced"`
	EmailsFailed     int              `json:"emails_failed"`
	EmailsQueued     int              `json:"emails_queued"`
	UniqueOpens      int              `json:"unique_opens"`
	OpenRate         float64          `json:"open_rate"`
	UniqueClicks     int              `json:"unique_clicks"`
	ClickThroughRate float64          `json:"click_through_rate"`
	Submissions      int              `json:"submissions"`
	SubmissionRate   float64          `json:"submission_rate"`
	Reports          int              `json:"reports"`
	ReportRate       float64          `json:"report_rate"`
	VariantMetrics   []VariantMetrics `json:"variant_metrics"`
}

// VariantMetrics holds per-variant metrics breakdown.
type VariantMetrics struct {
	VariantLabel     string  `json:"variant_label"`
	EmailsSent       int     `json:"emails_sent"`
	EmailsDelivered  int     `json:"emails_delivered"`
	EmailsBounced    int     `json:"emails_bounced"`
	EmailsFailed     int     `json:"emails_failed"`
	UniqueOpens      int     `json:"unique_opens"`
	OpenRate         float64 `json:"open_rate"`
	UniqueClicks     int     `json:"unique_clicks"`
	ClickThroughRate float64 `json:"click_through_rate"`
	Submissions      int     `json:"submissions"`
	SubmissionRate   float64 `json:"submission_rate"`
}

// HourlyBucket represents event counts in a one-hour window.
type HourlyBucket struct {
	Hour        time.Time `json:"hour"`
	Opens       int       `json:"opens"`
	Clicks      int       `json:"clicks"`
	Submissions int       `json:"submissions"`
	EmailsSent  int       `json:"emails_sent"`
}

// TimelineData holds time-series metrics.
type TimelineData struct {
	CampaignID string         `json:"campaign_id"`
	Buckets    []HourlyBucket `json:"buckets"`
}

// OrganizationMetrics holds aggregate metrics across all completed campaigns.
type OrganizationMetrics struct {
	TotalCampaigns      int     `json:"total_campaigns"`
	TotalTargets        int     `json:"total_targets"`
	TotalEmailsSent     int     `json:"total_emails_sent"`
	AvgOpenRate         float64 `json:"avg_open_rate"`
	AvgClickRate        float64 `json:"avg_click_rate"`
	AvgSubmissionRate   float64 `json:"avg_submission_rate"`
	SusceptibilityScore float64 `json:"susceptibility_score"`
}

// DepartmentMetrics holds metrics broken down by department.
type DepartmentMetrics struct {
	Department      string  `json:"department"`
	TargetCount     int     `json:"target_count"`
	EmailsSent      int     `json:"emails_sent"`
	UniqueOpens     int     `json:"unique_opens"`
	OpenRate        float64 `json:"open_rate"`
	UniqueClicks    int     `json:"unique_clicks"`
	ClickRate       float64 `json:"click_rate"`
	Submissions     int     `json:"submissions"`
	SubmissionRate  float64 `json:"submission_rate"`
	Susceptibility  float64 `json:"susceptibility"`
}

// TrendPoint represents a single data point in a trend line.
type TrendPoint struct {
	CampaignID   string    `json:"campaign_id"`
	CampaignName string    `json:"campaign_name"`
	CompletedAt  time.Time `json:"completed_at"`
	OpenRate     float64   `json:"open_rate"`
	ClickRate    float64   `json:"click_rate"`
	SubmitRate   float64   `json:"submit_rate"`
}

type cacheEntry struct {
	data      CampaignMetrics
	expiresAt time.Time
}

// Service provides campaign metrics aggregation.
type Service struct {
	db    *sql.DB
	mu    sync.RWMutex
	cache map[string]cacheEntry
}

// NewService creates a new metrics service.
func NewService(db *sql.DB) *Service {
	return &Service{
		db:    db,
		cache: make(map[string]cacheEntry),
	}
}

// GetCampaignMetrics returns computed metrics for a campaign with 30s caching.
func (s *Service) GetCampaignMetrics(ctx context.Context, campaignID string) (CampaignMetrics, error) {
	// Check cache.
	s.mu.RLock()
	if entry, ok := s.cache[campaignID]; ok && time.Now().Before(entry.expiresAt) {
		s.mu.RUnlock()
		return entry.data, nil
	}
	s.mu.RUnlock()

	m := CampaignMetrics{CampaignID: campaignID}

	// Total targets.
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM campaign_targets WHERE campaign_id = $1 AND removed_at IS NULL`,
		campaignID,
	).Scan(&m.TotalTargets); err != nil {
		return m, fmt.Errorf("metrics: total targets: %w", err)
	}

	// Email counts by status.
	rows, err := s.db.QueryContext(ctx,
		`SELECT status, COUNT(*) FROM campaign_emails WHERE campaign_id = $1 GROUP BY status`,
		campaignID)
	if err != nil {
		return m, fmt.Errorf("metrics: email counts: %w", err)
	}
	emailCounts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			rows.Close()
			return m, fmt.Errorf("metrics: email counts scan: %w", err)
		}
		emailCounts[status] = count
	}
	rows.Close()

	m.EmailsSent = emailCounts["sent"] + emailCounts["delivered"]
	m.EmailsDelivered = emailCounts["delivered"]
	m.EmailsBounced = emailCounts["bounced"]
	m.EmailsFailed = emailCounts["failed"]
	m.EmailsQueued = emailCounts["queued"]

	// Unique opens: count distinct target_id with email_opened events.
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT target_id) FROM campaign_target_events WHERE campaign_id = $1 AND event_type = 'email_opened'`,
		campaignID,
	).Scan(&m.UniqueOpens); err != nil {
		return m, fmt.Errorf("metrics: unique opens: %w", err)
	}

	// Unique clicks.
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT target_id) FROM campaign_target_events WHERE campaign_id = $1 AND event_type = 'link_clicked'`,
		campaignID,
	).Scan(&m.UniqueClicks); err != nil {
		return m, fmt.Errorf("metrics: unique clicks: %w", err)
	}

	// Submissions from capture_events.
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT target_id) FROM capture_events WHERE campaign_id = $1 AND target_id IS NOT NULL`,
		campaignID,
	).Scan(&m.Submissions); err != nil {
		return m, fmt.Errorf("metrics: submissions: %w", err)
	}

	// Reports.
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT target_id) FROM campaign_target_events WHERE campaign_id = $1 AND event_type = 'reported'`,
		campaignID,
	).Scan(&m.Reports); err != nil {
		return m, fmt.Errorf("metrics: reports: %w", err)
	}

	// Compute rates.
	delivered := m.EmailsSent
	if delivered == 0 {
		delivered = m.TotalTargets // fallback
	}
	if delivered > 0 {
		m.OpenRate = float64(m.UniqueOpens) / float64(delivered) * 100
		m.ClickThroughRate = float64(m.UniqueClicks) / float64(delivered) * 100
		m.ReportRate = float64(m.Reports) / float64(delivered) * 100
	}
	if m.UniqueClicks > 0 {
		m.SubmissionRate = float64(m.Submissions) / float64(m.UniqueClicks) * 100
	}

	// Per-variant breakdown.
	variants, err := s.getVariantMetrics(ctx, campaignID)
	if err != nil {
		return m, fmt.Errorf("metrics: variants: %w", err)
	}
	m.VariantMetrics = variants

	// Cache result.
	s.mu.Lock()
	s.cache[campaignID] = cacheEntry{data: m, expiresAt: time.Now().Add(30 * time.Second)}
	s.mu.Unlock()

	return m, nil
}

// GetTimeline returns hourly bucketed event counts for a campaign.
func (s *Service) GetTimeline(ctx context.Context, campaignID string) (TimelineData, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT date_trunc('hour', created_at) AS hour,
		       COUNT(*) FILTER (WHERE event_type = 'email_opened') AS opens,
		       COUNT(*) FILTER (WHERE event_type = 'link_clicked') AS clicks,
		       COUNT(*) FILTER (WHERE event_type = 'credential_submitted') AS submissions,
		       COUNT(*) FILTER (WHERE event_type = 'email_sent') AS emails_sent
		FROM campaign_target_events
		WHERE campaign_id = $1
		GROUP BY date_trunc('hour', created_at)
		ORDER BY hour`, campaignID)
	if err != nil {
		return TimelineData{}, fmt.Errorf("metrics: timeline: %w", err)
	}
	defer rows.Close()

	td := TimelineData{CampaignID: campaignID}
	for rows.Next() {
		var b HourlyBucket
		if err := rows.Scan(&b.Hour, &b.Opens, &b.Clicks, &b.Submissions, &b.EmailsSent); err != nil {
			return td, fmt.Errorf("metrics: timeline scan: %w", err)
		}
		td.Buckets = append(td.Buckets, b)
	}
	return td, rows.Err()
}

// GetOrganizationMetrics returns aggregate metrics across all completed campaigns.
func (s *Service) GetOrganizationMetrics(ctx context.Context) (OrganizationMetrics, error) {
	var om OrganizationMetrics

	err := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(DISTINCT c.id),
			COALESCE(SUM(sub.targets), 0),
			COALESCE(SUM(sub.sent), 0),
			CASE WHEN SUM(sub.sent) > 0 THEN SUM(sub.opens)::float / SUM(sub.sent) * 100 ELSE 0 END,
			CASE WHEN SUM(sub.sent) > 0 THEN SUM(sub.clicks)::float / SUM(sub.sent) * 100 ELSE 0 END,
			CASE WHEN SUM(sub.clicks) > 0 THEN SUM(sub.submissions)::float / SUM(sub.clicks) * 100 ELSE 0 END
		FROM campaigns c
		LEFT JOIN LATERAL (
			SELECT
				(SELECT COUNT(*) FROM campaign_targets ct WHERE ct.campaign_id = c.id AND ct.removed_at IS NULL) AS targets,
				(SELECT COUNT(*) FROM campaign_emails ce WHERE ce.campaign_id = c.id AND ce.status IN ('sent', 'delivered')) AS sent,
				(SELECT COUNT(DISTINCT cte.target_id) FROM campaign_target_events cte WHERE cte.campaign_id = c.id AND cte.event_type = 'email_opened') AS opens,
				(SELECT COUNT(DISTINCT cte2.target_id) FROM campaign_target_events cte2 WHERE cte2.campaign_id = c.id AND cte2.event_type = 'link_clicked') AS clicks,
				(SELECT COUNT(DISTINCT cap.target_id) FROM capture_events cap WHERE cap.campaign_id = c.id AND cap.target_id IS NOT NULL) AS submissions
		) sub ON true
		WHERE c.current_state IN ('completed', 'archived') AND c.deleted_at IS NULL`,
	).Scan(&om.TotalCampaigns, &om.TotalTargets, &om.TotalEmailsSent,
		&om.AvgOpenRate, &om.AvgClickRate, &om.AvgSubmissionRate)
	if err != nil {
		return om, fmt.Errorf("metrics: organization: %w", err)
	}

	// Susceptibility = click_rate * 0.4 + submission_rate * 0.6
	om.SusceptibilityScore = om.AvgClickRate*0.4 + om.AvgSubmissionRate*0.6

	return om, nil
}

// GetDepartmentMetrics returns metrics broken down by target department.
func (s *Service) GetDepartmentMetrics(ctx context.Context) ([]DepartmentMetrics, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			COALESCE(t.department, 'Unknown') AS dept,
			COUNT(DISTINCT t.id) AS target_count,
			COUNT(DISTINCT ce.id) FILTER (WHERE ce.status IN ('sent', 'delivered')) AS emails_sent,
			COUNT(DISTINCT cte_open.target_id) AS unique_opens,
			COUNT(DISTINCT cte_click.target_id) AS unique_clicks,
			COUNT(DISTINCT cap.target_id) AS submissions
		FROM targets t
		JOIN campaign_targets ct ON ct.target_id = t.id AND ct.removed_at IS NULL
		JOIN campaigns c ON c.id = ct.campaign_id AND c.current_state IN ('completed', 'archived') AND c.deleted_at IS NULL
		LEFT JOIN campaign_emails ce ON ce.campaign_id = c.id AND ce.target_id = t.id
		LEFT JOIN campaign_target_events cte_open ON cte_open.campaign_id = c.id AND cte_open.target_id = t.id AND cte_open.event_type = 'email_opened'
		LEFT JOIN campaign_target_events cte_click ON cte_click.campaign_id = c.id AND cte_click.target_id = t.id AND cte_click.event_type = 'link_clicked'
		LEFT JOIN capture_events cap ON cap.campaign_id = c.id AND cap.target_id = t.id
		WHERE t.deleted_at IS NULL
		GROUP BY COALESCE(t.department, 'Unknown')
		ORDER BY dept`)
	if err != nil {
		return nil, fmt.Errorf("metrics: departments: %w", err)
	}
	defer rows.Close()

	var results []DepartmentMetrics
	for rows.Next() {
		var d DepartmentMetrics
		if err := rows.Scan(&d.Department, &d.TargetCount, &d.EmailsSent,
			&d.UniqueOpens, &d.UniqueClicks, &d.Submissions); err != nil {
			return nil, fmt.Errorf("metrics: departments scan: %w", err)
		}
		if d.EmailsSent > 0 {
			d.OpenRate = float64(d.UniqueOpens) / float64(d.EmailsSent) * 100
			d.ClickRate = float64(d.UniqueClicks) / float64(d.EmailsSent) * 100
		}
		if d.UniqueClicks > 0 {
			d.SubmissionRate = float64(d.Submissions) / float64(d.UniqueClicks) * 100
		}
		d.Susceptibility = d.ClickRate*0.4 + d.SubmissionRate*0.6
		results = append(results, d)
	}
	return results, rows.Err()
}

// GetTrends returns susceptibility trends over time (one point per completed campaign).
func (s *Service) GetTrends(ctx context.Context) ([]TrendPoint, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.name, COALESCE(c.completed_at, c.updated_at),
			sub.open_rate, sub.click_rate, sub.submit_rate
		FROM campaigns c
		LEFT JOIN LATERAL (
			SELECT
				CASE WHEN sent > 0 THEN opens::float / sent * 100 ELSE 0 END AS open_rate,
				CASE WHEN sent > 0 THEN clicks::float / sent * 100 ELSE 0 END AS click_rate,
				CASE WHEN clicks > 0 THEN submissions::float / clicks * 100 ELSE 0 END AS submit_rate
			FROM (
				SELECT
					(SELECT COUNT(*) FROM campaign_emails ce WHERE ce.campaign_id = c.id AND ce.status IN ('sent', 'delivered')) AS sent,
					(SELECT COUNT(DISTINCT cte.target_id) FROM campaign_target_events cte WHERE cte.campaign_id = c.id AND cte.event_type = 'email_opened') AS opens,
					(SELECT COUNT(DISTINCT cte2.target_id) FROM campaign_target_events cte2 WHERE cte2.campaign_id = c.id AND cte2.event_type = 'link_clicked') AS clicks,
					(SELECT COUNT(DISTINCT cap.target_id) FROM capture_events cap WHERE cap.campaign_id = c.id AND cap.target_id IS NOT NULL) AS submissions
			) sq
		) sub ON true
		WHERE c.current_state IN ('completed', 'archived') AND c.deleted_at IS NULL
		ORDER BY COALESCE(c.completed_at, c.updated_at)`)
	if err != nil {
		return nil, fmt.Errorf("metrics: trends: %w", err)
	}
	defer rows.Close()

	var results []TrendPoint
	for rows.Next() {
		var p TrendPoint
		if err := rows.Scan(&p.CampaignID, &p.CampaignName, &p.CompletedAt,
			&p.OpenRate, &p.ClickRate, &p.SubmitRate); err != nil {
			return nil, fmt.Errorf("metrics: trends scan: %w", err)
		}
		results = append(results, p)
	}
	return results, rows.Err()
}

func (s *Service) getVariantMetrics(ctx context.Context, campaignID string) ([]VariantMetrics, error) {
	// Get variant labels for this campaign.
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT COALESCE(variant_label, 'default') FROM campaign_emails WHERE campaign_id = $1`,
		campaignID)
	if err != nil {
		return nil, fmt.Errorf("metrics: variant labels: %w", err)
	}
	defer rows.Close()

	var labels []string
	for rows.Next() {
		var label string
		if err := rows.Scan(&label); err != nil {
			return nil, err
		}
		labels = append(labels, label)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(labels) <= 1 {
		return nil, nil // No variant breakdown needed for single variant.
	}

	var variants []VariantMetrics
	for _, label := range labels {
		vm := VariantMetrics{VariantLabel: label}

		// Email counts for this variant.
		variantRows, err := s.db.QueryContext(ctx,
			`SELECT status, COUNT(*) FROM campaign_emails
			 WHERE campaign_id = $1 AND COALESCE(variant_label, 'default') = $2
			 GROUP BY status`, campaignID, label)
		if err != nil {
			return nil, fmt.Errorf("metrics: variant email counts: %w", err)
		}
		for variantRows.Next() {
			var status string
			var count int
			if err := variantRows.Scan(&status, &count); err != nil {
				variantRows.Close()
				return nil, err
			}
			switch status {
			case "sent", "delivered":
				vm.EmailsSent += count
				if status == "delivered" {
					vm.EmailsDelivered = count
				}
			case "bounced":
				vm.EmailsBounced = count
			case "failed":
				vm.EmailsFailed = count
			}
		}
		variantRows.Close()

		// Get target IDs for this variant to count opens/clicks/submissions.
		targetRows, err := s.db.QueryContext(ctx,
			`SELECT DISTINCT target_id FROM campaign_emails
			 WHERE campaign_id = $1 AND COALESCE(variant_label, 'default') = $2`,
			campaignID, label)
		if err != nil {
			return nil, fmt.Errorf("metrics: variant targets: %w", err)
		}
		var targetIDs []string
		for targetRows.Next() {
			var tid string
			if err := targetRows.Scan(&tid); err != nil {
				targetRows.Close()
				return nil, err
			}
			targetIDs = append(targetIDs, tid)
		}
		targetRows.Close()

		if len(targetIDs) > 0 {
			// Unique opens for variant targets.
			if err := s.db.QueryRowContext(ctx, `
				SELECT COUNT(DISTINCT target_id) FROM campaign_target_events
				WHERE campaign_id = $1 AND event_type = 'email_opened'
				  AND target_id = ANY(
				    SELECT DISTINCT target_id FROM campaign_emails
				    WHERE campaign_id = $1 AND COALESCE(variant_label, 'default') = $2
				  )`, campaignID, label).Scan(&vm.UniqueOpens); err != nil {
				return nil, fmt.Errorf("metrics: variant opens: %w", err)
			}

			// Unique clicks for variant targets.
			if err := s.db.QueryRowContext(ctx, `
				SELECT COUNT(DISTINCT target_id) FROM campaign_target_events
				WHERE campaign_id = $1 AND event_type = 'link_clicked'
				  AND target_id = ANY(
				    SELECT DISTINCT target_id FROM campaign_emails
				    WHERE campaign_id = $1 AND COALESCE(variant_label, 'default') = $2
				  )`, campaignID, label).Scan(&vm.UniqueClicks); err != nil {
				return nil, fmt.Errorf("metrics: variant clicks: %w", err)
			}

			// Submissions for variant targets.
			if err := s.db.QueryRowContext(ctx, `
				SELECT COUNT(DISTINCT target_id) FROM capture_events
				WHERE campaign_id = $1 AND target_id IS NOT NULL
				  AND target_id = ANY(
				    SELECT DISTINCT target_id FROM campaign_emails
				    WHERE campaign_id = $1 AND COALESCE(variant_label, 'default') = $2
				  )`, campaignID, label).Scan(&vm.Submissions); err != nil {
				return nil, fmt.Errorf("metrics: variant submissions: %w", err)
			}
		}

		// Compute rates.
		sent := vm.EmailsSent
		if sent > 0 {
			vm.OpenRate = float64(vm.UniqueOpens) / float64(sent) * 100
			vm.ClickThroughRate = float64(vm.UniqueClicks) / float64(sent) * 100
		}
		if vm.UniqueClicks > 0 {
			vm.SubmissionRate = float64(vm.Submissions) / float64(vm.UniqueClicks) * 100
		}

		variants = append(variants, vm)
	}

	return variants, nil
}
