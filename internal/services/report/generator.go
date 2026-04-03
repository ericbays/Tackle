package report

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	metricssvc "tackle/internal/services/metrics"
)

// Generator handles async report generation.
type Generator struct {
	db         *sql.DB
	metricsSvc *metricssvc.Service
	outputDir  string
	logger     *slog.Logger
}

// NewGenerator creates a new report generator.
func NewGenerator(db *sql.DB, metricsSvc *metricssvc.Service, outputDir string, logger *slog.Logger) *Generator {
	return &Generator{
		db:         db,
		metricsSvc: metricsSvc,
		outputDir:  outputDir,
		logger:     logger,
	}
}

// GenerateInput holds input for report generation.
type GenerateInput struct {
	TemplateID  string         `json:"template_id"`
	CampaignIDs []string       `json:"campaign_ids"`
	Title       string         `json:"title"`
	Format      string         `json:"format"`
	Parameters  map[string]any `json:"parameters"`
}

// GenerateReport creates a report record and starts async generation.
func (g *Generator) GenerateReport(ctx context.Context, input GenerateInput, userID string) (GeneratedReport, error) {
	id := uuid.New().String()

	paramsJSON := mapToJSON(input.Parameters)

	// Validate format.
	switch input.Format {
	case "csv", "json", "html":
		// OK
	default:
		input.Format = "csv" // Default to CSV.
	}

	var report GeneratedReport
	var campaignIDsStr string
	err := g.db.QueryRowContext(ctx, `
		INSERT INTO generated_reports (id, template_id, campaign_ids, title, format, status, parameters, generated_by)
		VALUES ($1, $2, $3::uuid[], $4, $5, 'pending', $6, $7)
		RETURNING id, template_id, campaign_ids::text, title, format, status, parameters,
		          error_message, generated_by, started_at, completed_at, created_at, updated_at`,
		id, nilIfEmpty(input.TemplateID), uuidArray(input.CampaignIDs), input.Title, input.Format, paramsJSON, userID,
	).Scan(&report.ID, &report.TemplateID, &campaignIDsStr, &report.Title, &report.Format, &report.Status,
		&[]byte{}, &report.ErrorMessage, &report.GeneratedBy,
		&report.StartedAt, &report.CompletedAt, &report.CreatedAt, &report.UpdatedAt)
	if err != nil {
		return report, fmt.Errorf("report generator: create: %w", err)
	}
	report.CampaignIDs = input.CampaignIDs

	// Start async generation.
	go g.generate(id, input)

	return report, nil
}

// ListReports returns all generated reports for a user.
func (g *Generator) ListReports(ctx context.Context) ([]GeneratedReport, error) {
	rows, err := g.db.QueryContext(ctx, `
		SELECT id, template_id, campaign_ids::text, title, format, status, file_path, file_size_bytes,
		       error_message, generated_by, started_at, completed_at, created_at, updated_at
		FROM generated_reports ORDER BY created_at DESC LIMIT 100`)
	if err != nil {
		return nil, fmt.Errorf("reports: list: %w", err)
	}
	defer rows.Close()

	var reports []GeneratedReport
	for rows.Next() {
		var r GeneratedReport
		var campaignIDsStr string
		if err := rows.Scan(&r.ID, &r.TemplateID, &campaignIDsStr, &r.Title, &r.Format, &r.Status,
			&r.FilePath, &r.FileSizeBytes, &r.ErrorMessage, &r.GeneratedBy,
			&r.StartedAt, &r.CompletedAt, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("reports: list scan: %w", err)
		}
		r.CampaignIDs = parseUUIDArray(campaignIDsStr)
		reports = append(reports, r)
	}
	return reports, rows.Err()
}

// GetReport returns a single generated report.
func (g *Generator) GetReport(ctx context.Context, id string) (GeneratedReport, error) {
	var r GeneratedReport
	var campaignIDsStr string
	err := g.db.QueryRowContext(ctx, `
		SELECT id, template_id, campaign_ids::text, title, format, status, file_path, file_size_bytes,
		       error_message, generated_by, started_at, completed_at, created_at, updated_at
		FROM generated_reports WHERE id = $1`, id,
	).Scan(&r.ID, &r.TemplateID, &campaignIDsStr, &r.Title, &r.Format, &r.Status,
		&r.FilePath, &r.FileSizeBytes, &r.ErrorMessage, &r.GeneratedBy,
		&r.StartedAt, &r.CompletedAt, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return r, fmt.Errorf("reports: get: %w", err)
	}
	r.CampaignIDs = parseUUIDArray(campaignIDsStr)
	return r, nil
}

// DeleteReport deletes a generated report and its file.
func (g *Generator) DeleteReport(ctx context.Context, id string) error {
	// Get the file path first.
	var filePath *string
	_ = g.db.QueryRowContext(ctx, `SELECT file_path FROM generated_reports WHERE id = $1`, id).Scan(&filePath)

	_, err := g.db.ExecContext(ctx, `DELETE FROM generated_reports WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("reports: delete: %w", err)
	}

	// Clean up file.
	if filePath != nil && *filePath != "" {
		_ = os.Remove(*filePath)
	}
	return nil
}

// GetFilePath returns the file path for a generated report (for downloads).
func (g *Generator) GetFilePath(ctx context.Context, id string) (string, string, error) {
	var filePath *string
	var format string
	err := g.db.QueryRowContext(ctx,
		`SELECT file_path, format FROM generated_reports WHERE id = $1 AND status = 'completed'`, id,
	).Scan(&filePath, &format)
	if err != nil {
		return "", "", fmt.Errorf("reports: get file path: %w", err)
	}
	if filePath == nil || *filePath == "" {
		return "", "", fmt.Errorf("reports: no file for report %s", id)
	}
	return *filePath, format, nil
}

func (g *Generator) generate(reportID string, input GenerateInput) {
	ctx := context.Background()

	// Mark as generating.
	now := time.Now()
	_, _ = g.db.ExecContext(ctx,
		`UPDATE generated_reports SET status = 'generating', started_at = $2 WHERE id = $1`,
		reportID, now)

	// Collect metrics for each campaign.
	var allMetrics []metricssvc.CampaignMetrics
	for _, cid := range input.CampaignIDs {
		m, err := g.metricsSvc.GetCampaignMetrics(ctx, cid)
		if err != nil {
			g.failReport(ctx, reportID, fmt.Sprintf("failed to get metrics for campaign %s: %v", cid, err))
			return
		}
		allMetrics = append(allMetrics, m)
	}

	// Ensure output directory exists.
	if err := os.MkdirAll(g.outputDir, 0750); err != nil {
		g.failReport(ctx, reportID, fmt.Sprintf("failed to create output directory: %v", err))
		return
	}

	var filePath string
	var err error

	switch input.Format {
	case "csv":
		filePath, err = g.generateCSV(reportID, allMetrics)
	case "json":
		filePath, err = g.generateJSON(reportID, allMetrics)
	case "html":
		filePath, err = g.generateHTML(reportID, input.Title, allMetrics)
	default:
		filePath, err = g.generateCSV(reportID, allMetrics)
	}

	if err != nil {
		g.failReport(ctx, reportID, err.Error())
		return
	}

	// Get file size.
	info, _ := os.Stat(filePath)
	var fileSize int64
	if info != nil {
		fileSize = info.Size()
	}

	completed := time.Now()
	_, _ = g.db.ExecContext(ctx,
		`UPDATE generated_reports SET status = 'completed', file_path = $2, file_size_bytes = $3, completed_at = $4 WHERE id = $1`,
		reportID, filePath, fileSize, completed)

	g.logger.Info("report generated", "report_id", reportID, "format", input.Format, "size", fileSize)
}

func (g *Generator) generateCSV(reportID string, metrics []metricssvc.CampaignMetrics) (string, error) {
	filePath := filepath.Join(g.outputDir, reportID+".csv")
	f, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("create csv: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	// Header.
	header := []string{
		"Campaign ID", "Total Targets", "Emails Sent", "Emails Delivered",
		"Emails Bounced", "Emails Failed", "Unique Opens", "Open Rate (%)",
		"Unique Clicks", "CTR (%)", "Submissions", "Submission Rate (%)",
		"Reports", "Report Rate (%)",
	}
	if err := w.Write(header); err != nil {
		return "", fmt.Errorf("write csv header: %w", err)
	}

	for _, m := range metrics {
		row := []string{
			m.CampaignID,
			fmt.Sprintf("%d", m.TotalTargets),
			fmt.Sprintf("%d", m.EmailsSent),
			fmt.Sprintf("%d", m.EmailsDelivered),
			fmt.Sprintf("%d", m.EmailsBounced),
			fmt.Sprintf("%d", m.EmailsFailed),
			fmt.Sprintf("%d", m.UniqueOpens),
			fmt.Sprintf("%.1f", m.OpenRate),
			fmt.Sprintf("%d", m.UniqueClicks),
			fmt.Sprintf("%.1f", m.ClickThroughRate),
			fmt.Sprintf("%d", m.Submissions),
			fmt.Sprintf("%.1f", m.SubmissionRate),
			fmt.Sprintf("%d", m.Reports),
			fmt.Sprintf("%.1f", m.ReportRate),
		}
		if err := w.Write(row); err != nil {
			return "", fmt.Errorf("write csv row: %w", err)
		}
	}

	return filePath, nil
}

func (g *Generator) generateJSON(reportID string, metrics []metricssvc.CampaignMetrics) (string, error) {
	filePath := filepath.Join(g.outputDir, reportID+".json")
	f, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("create json: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(map[string]any{
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"campaigns":    metrics,
	}); err != nil {
		return "", fmt.Errorf("write json: %w", err)
	}

	return filePath, nil
}

func (g *Generator) generateHTML(reportID, title string, metrics []metricssvc.CampaignMetrics) (string, error) {
	filePath := filepath.Join(g.outputDir, reportID+".html")
	f, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("create html: %w", err)
	}
	defer f.Close()

	// Simple HTML report.
	fmt.Fprintf(f, `<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>%s</title>
<style>
body{font-family:system-ui,sans-serif;margin:2rem;color:#1a1a1a}
table{border-collapse:collapse;width:100%%}
th,td{border:1px solid #ddd;padding:8px 12px;text-align:right}
th{background:#f5f5f5;text-align:left}
h1{color:#1a1a1a}h2{color:#444}
.metric{display:inline-block;padding:1rem;margin:0.5rem;background:#f9f9f9;border-radius:8px;min-width:120px;text-align:center}
.metric-value{font-size:2rem;font-weight:bold;color:#2563eb}
.metric-label{font-size:0.85rem;color:#666}
</style></head><body>`, title)

	fmt.Fprintf(f, "<h1>%s</h1>\n", title)
	fmt.Fprintf(f, "<p>Generated: %s</p>\n", time.Now().UTC().Format("2006-01-02 15:04:05 UTC"))

	for _, m := range metrics {
		fmt.Fprintf(f, "<h2>Campaign: %s</h2>\n", m.CampaignID)
		fmt.Fprint(f, `<div>`)
		writeMetricCard(f, "Targets", fmt.Sprintf("%d", m.TotalTargets))
		writeMetricCard(f, "Emails Sent", fmt.Sprintf("%d", m.EmailsSent))
		writeMetricCard(f, "Open Rate", fmt.Sprintf("%.1f%%", m.OpenRate))
		writeMetricCard(f, "Click Rate", fmt.Sprintf("%.1f%%", m.ClickThroughRate))
		writeMetricCard(f, "Submit Rate", fmt.Sprintf("%.1f%%", m.SubmissionRate))
		writeMetricCard(f, "Report Rate", fmt.Sprintf("%.1f%%", m.ReportRate))
		fmt.Fprint(f, `</div>`)

		if len(m.VariantMetrics) > 0 {
			fmt.Fprint(f, `<h3>Variant Breakdown</h3><table>`)
			fmt.Fprint(f, `<tr><th>Variant</th><th>Sent</th><th>Opens</th><th>Open Rate</th><th>Clicks</th><th>CTR</th><th>Submissions</th><th>Submit Rate</th></tr>`)
			for _, v := range m.VariantMetrics {
				fmt.Fprintf(f, "<tr><td>%s</td><td>%d</td><td>%d</td><td>%.1f%%</td><td>%d</td><td>%.1f%%</td><td>%d</td><td>%.1f%%</td></tr>\n",
					v.VariantLabel, v.EmailsSent, v.UniqueOpens, v.OpenRate, v.UniqueClicks, v.ClickThroughRate, v.Submissions, v.SubmissionRate)
			}
			fmt.Fprint(f, `</table>`)
		}
	}

	fmt.Fprint(f, `</body></html>`)
	return filePath, nil
}

func writeMetricCard(f *os.File, label, value string) {
	fmt.Fprintf(f, `<div class="metric"><div class="metric-value">%s</div><div class="metric-label">%s</div></div>`, value, label)
}

func (g *Generator) failReport(ctx context.Context, reportID, errMsg string) {
	g.logger.Error("report generation failed", "report_id", reportID, "error", errMsg)
	_, _ = g.db.ExecContext(ctx,
		`UPDATE generated_reports SET status = 'failed', error_message = $2, completed_at = $3 WHERE id = $1`,
		reportID, errMsg, time.Now())
}
