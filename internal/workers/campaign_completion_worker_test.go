package workers

import (
	"testing"
	"time"

	campaignsvc "tackle/internal/services/campaign"
)

func TestShouldComplete_EndDatePassed(t *testing.T) {
	w := &CampaignCompletionWorker{}
	pastEnd := time.Now().UTC().Add(-1 * time.Hour)
	c := campaignsvc.CampaignDTO{
		EndDate:        &pastEnd,
		StateChangedAt: time.Now().UTC().Add(-2 * time.Hour),
	}

	if !w.shouldComplete(c, time.Now().UTC()) {
		t.Error("expected shouldComplete=true when end_date is in the past")
	}
}

func TestShouldComplete_EndDateNotPassed(t *testing.T) {
	w := &CampaignCompletionWorker{}
	futureEnd := time.Now().UTC().Add(24 * time.Hour)
	c := campaignsvc.CampaignDTO{
		EndDate:          &futureEnd,
		GracePeriodHours: 1,
		StateChangedAt:   time.Now().UTC(), // just became active
	}

	if w.shouldComplete(c, time.Now().UTC()) {
		t.Error("expected shouldComplete=false when end_date is in the future and grace period not elapsed")
	}
}

func TestShouldComplete_GracePeriodElapsed(t *testing.T) {
	w := &CampaignCompletionWorker{}
	// No end_date, but grace period of 1 hour elapsed.
	c := campaignsvc.CampaignDTO{
		EndDate:          nil,
		GracePeriodHours: 1,
		StateChangedAt:   time.Now().UTC().Add(-2 * time.Hour),
	}

	if !w.shouldComplete(c, time.Now().UTC()) {
		t.Error("expected shouldComplete=true when grace period has elapsed")
	}
}

func TestShouldComplete_GracePeriodNotElapsed(t *testing.T) {
	w := &CampaignCompletionWorker{}
	// No end_date, grace period of 72 hours, only 1 hour in.
	c := campaignsvc.CampaignDTO{
		EndDate:          nil,
		GracePeriodHours: 72,
		StateChangedAt:   time.Now().UTC().Add(-1 * time.Hour),
	}

	if w.shouldComplete(c, time.Now().UTC()) {
		t.Error("expected shouldComplete=false when grace period has not elapsed")
	}
}

func TestShouldComplete_DefaultGracePeriod(t *testing.T) {
	w := &CampaignCompletionWorker{}
	// No end_date, no grace_period_hours (0 → defaults to 72h), 73 hours ago.
	c := campaignsvc.CampaignDTO{
		EndDate:          nil,
		GracePeriodHours: 0,
		StateChangedAt:   time.Now().UTC().Add(-73 * time.Hour),
	}

	if !w.shouldComplete(c, time.Now().UTC()) {
		t.Error("expected shouldComplete=true when default 72h grace period has elapsed")
	}
}
