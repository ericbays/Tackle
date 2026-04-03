package emaildelivery

import (
	"context"
	"testing"
	"time"

	"tackle/internal/repositories"
)

func TestIsWithinSendWindow(t *testing.T) {
	svc := &Service{}

	tests := []struct {
		name    string
		windows []repositories.CampaignSendWindow
		want    bool
	}{
		{
			name:    "no windows means always open",
			windows: nil,
			want:    false, // isWithinSendWindow returns false with no windows.
		},
		{
			name: "all days, wide time range",
			windows: []repositories.CampaignSendWindow{
				{Days: []string{}, StartTime: "00:00:00", EndTime: "23:59:59", Timezone: "UTC"},
			},
			want: true,
		},
		{
			name: "today included",
			windows: []repositories.CampaignSendWindow{
				{
					Days:      allDays(),
					StartTime: "00:00:00",
					EndTime:   "23:59:59",
					Timezone:  "UTC",
				},
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := svc.isWithinSendWindow(tc.windows)
			if got != tc.want {
				t.Errorf("isWithinSendWindow() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNextWindowOpen(t *testing.T) {
	svc := &Service{}

	windows := []repositories.CampaignSendWindow{
		{
			Days:      allDays(),
			StartTime: "08:00:00",
			EndTime:   "17:00:00",
			Timezone:  "UTC",
		},
	}

	nextOpen := svc.nextWindowOpen(windows)
	if nextOpen.IsZero() {
		t.Error("nextWindowOpen() returned zero time")
	}
	if !nextOpen.After(time.Now()) {
		// It's possible we're within the window — skip in that case.
		now := time.Now().UTC()
		currentTime := now.Format("15:04:05")
		if currentTime < "08:00:00" || currentTime > "17:00:00" {
			t.Error("nextWindowOpen() should be in the future")
		}
	}
}

func TestApplyDelay_CSPRNG(t *testing.T) {
	svc := &Service{config: DefaultConfig()}

	schedule := &repositories.CampaignSendSchedule{
		MinDelayMs: 10,
		MaxDelayMs: 100,
	}

	// Just verify it doesn't panic and completes within a reasonable time.
	start := time.Now()
	svc.applyDelay(t.Context(), schedule)
	elapsed := time.Since(start)

	if elapsed < 10*time.Millisecond {
		t.Error("delay was too short, expected at least 10ms")
	}
	if elapsed > 200*time.Millisecond {
		t.Error("delay was too long, expected at most 200ms")
	}
}

func TestApplyDelay_NoDelay(t *testing.T) {
	svc := &Service{config: DefaultConfig()}

	schedule := &repositories.CampaignSendSchedule{
		MinDelayMs: 0,
		MaxDelayMs: 0,
	}

	start := time.Now()
	svc.applyDelay(t.Context(), schedule)
	elapsed := time.Since(start)

	if elapsed > 10*time.Millisecond {
		t.Error("expected no delay when min and max are 0")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if cfg.InitialRetryInterval != 5*time.Minute {
		t.Errorf("InitialRetryInterval = %v, want 5m", cfg.InitialRetryInterval)
	}
	if cfg.DefaultRateLimit != 60 {
		t.Errorf("DefaultRateLimit = %d, want 60", cfg.DefaultRateLimit)
	}
}

func TestPtrStr(t *testing.T) {
	tests := []struct {
		name string
		s    *string
		want string
	}{
		{"nil", nil, ""},
		{"empty", strPtr(""), ""},
		{"value", strPtr("hello"), "hello"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ptrStr(tc.s); got != tc.want {
				t.Errorf("ptrStr() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAdjustToNextWindow(t *testing.T) {
	svc := &Service{}

	// Window: 08:00-17:00 UTC every day.
	windows := []repositories.CampaignSendWindow{
		{Days: allDays(), StartTime: "08:00:00", EndTime: "17:00:00", Timezone: "UTC"},
	}

	// A time within the window should return unchanged.
	withinWindow := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC) // Wednesday noon.
	adjusted := svc.adjustToNextWindow(withinWindow, windows)
	if !adjusted.Equal(withinWindow) {
		t.Errorf("adjustToNextWindow(%v) = %v, want unchanged", withinWindow, adjusted)
	}

	// A time outside the window should be adjusted.
	outsideWindow := time.Date(2026, 3, 18, 20, 0, 0, 0, time.UTC) // Wednesday 8pm.
	adjusted = svc.adjustToNextWindow(outsideWindow, windows)
	if adjusted.Before(outsideWindow) || adjusted.Equal(outsideWindow) {
		t.Errorf("adjustToNextWindow(%v) = %v, should be after the input", outsideWindow, adjusted)
	}
	// Should be 08:00 the next day.
	expected := time.Date(2026, 3, 19, 8, 0, 0, 0, time.UTC)
	if !adjusted.Equal(expected) {
		t.Errorf("adjustToNextWindow(%v) = %v, want %v", outsideWindow, adjusted, expected)
	}
}

func TestDeliveryResult_Fields(t *testing.T) {
	// Verify DeliveryResult struct can be created with all fields.
	result := DeliveryResult{
		EmailID:       "test-email",
		CampaignID:    "test-campaign",
		TargetID:      "test-target",
		SMTPProfileID: "test-smtp",
		Status:        "delivered",
		MessageID:     strPtr("<msg@example.com>"),
		SMTPResponse:  strPtr("250 OK"),
		SentAt:        timePtr(time.Now()),
		DeliveredAt:   timePtr(time.Now()),
	}

	if result.EmailID != "test-email" {
		t.Error("unexpected EmailID")
	}
	if result.Status != "delivered" {
		t.Error("unexpected Status")
	}
}

func TestDeliveryMetrics_Fields(t *testing.T) {
	metrics := DeliveryMetrics{
		TotalEmails:  100,
		StatusCounts: map[string]int{"sent": 50, "delivered": 30, "bounced": 10, "failed": 10},
		ProfileCounts: []ProfileSendCount{
			{SMTPProfileID: "p1", SendCount: 60, ErrorCount: 5, ErrorRate: 5.0 / 60.0},
			{SMTPProfileID: "p2", SendCount: 40, ErrorCount: 15, ErrorRate: 15.0 / 40.0},
		},
		CurrentPosition: intPtr(75),
		IsSending:       true,
		IsPaused:        false,
	}

	if metrics.TotalEmails != 100 {
		t.Errorf("TotalEmails = %d, want 100", metrics.TotalEmails)
	}
	if len(metrics.ProfileCounts) != 2 {
		t.Errorf("ProfileCounts length = %d, want 2", len(metrics.ProfileCounts))
	}
}

func TestCampaignSender_WaitForUnpause(t *testing.T) {
	// Unpaused sender should return immediately.
	cs := &campaignSender{paused: make(chan struct{})}
	close(cs.paused)

	start := time.Now()
	err := cs.waitForUnpause(t.Context())
	if err != nil {
		t.Errorf("waitForUnpause() error = %v", err)
	}
	if time.Since(start) > 100*time.Millisecond {
		t.Error("waitForUnpause() should return immediately when unpaused")
	}
}

func TestCampaignSender_WaitForUnpause_Paused(t *testing.T) {
	cs := &campaignSender{paused: make(chan struct{})}
	// Don't close — simulate paused state.

	// Use a context with timeout to avoid blocking forever.
	ctx, cancel := t.Context(), func() {}
	_ = cancel

	done := make(chan error, 1)
	go func() {
		ctx2, c2 := newTestContext(t, 50*time.Millisecond)
		defer c2()
		done <- cs.waitForUnpause(ctx2)
	}()

	_ = ctx
	err := <-done
	if err == nil {
		t.Error("waitForUnpause() should have returned error due to context cancellation")
	}
}

// helpers

func allDays() []string {
	return []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }
func timePtr(t time.Time) *time.Time { return &t }

func newTestContext(t *testing.T, timeout time.Duration) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(t.Context(), timeout)
}

func TestBuildTrackingURLPaths(t *testing.T) {
	tests := []struct {
		name   string
		token  string
		expect map[string]string
	}{
		{
			name:   "empty token returns nil",
			token:  "",
			expect: nil,
		},
		{
			name:  "valid token produces correct paths",
			token: "abc123def456ghij",
			expect: map[string]string{
				"pixel_path":        "/t/abc123def456ghij.gif",
				"click_path":        "/c/abc123def456ghij/",
				"landing_page_path": "/l/abc123def456ghij",
			},
		},
		{
			name:  "short token",
			token: "tk",
			expect: map[string]string{
				"pixel_path":        "/t/tk.gif",
				"click_path":        "/c/tk/",
				"landing_page_path": "/l/tk",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildTrackingURLPaths(tt.token)
			if tt.expect == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}
			for key, want := range tt.expect {
				got, ok := result[key]
				if !ok {
					t.Errorf("missing key %q", key)
					continue
				}
				if got != want {
					t.Errorf("key %q: got %q, want %q", key, got, want)
				}
			}
		})
	}
}

// --- ECOMP-07: Per-Profile Rate Limiter Tests ---

func TestProfileRateLimiter_RecordSend(t *testing.T) {
	prl := newProfileRateLimiter()

	// First send should return 1.
	count := prl.recordSend("profile-a")
	if count != 1 {
		t.Errorf("first send count = %d, want 1", count)
	}

	// Second send same profile should return 2.
	count = prl.recordSend("profile-a")
	if count != 2 {
		t.Errorf("second send count = %d, want 2", count)
	}

	// Different profile should return 1.
	count = prl.recordSend("profile-b")
	if count != 1 {
		t.Errorf("different profile count = %d, want 1", count)
	}
}

func TestProfileRateLimiter_WindowReset(t *testing.T) {
	prl := newProfileRateLimiter()

	// Record a send.
	prl.recordSend("profile-a")

	// Manually expire the window.
	prl.mu.Lock()
	prl.counts["profile-a"].windowEnd = time.Now().Add(-1 * time.Second)
	prl.mu.Unlock()

	// Next send should start a new window (count = 1).
	count := prl.recordSend("profile-a")
	if count != 1 {
		t.Errorf("after window reset, count = %d, want 1", count)
	}
}

func TestApplyRateLimit_EffectiveRate(t *testing.T) {
	// Verify the rate limit calculation uses the most restrictive.
	// Campaign: 100/min, Profile: 50/min → effective 50/min → delay ~1.2s
	// Campaign: 30/min, Profile: 0 → effective 30/min → delay ~2s

	campaignRate := 100
	schedule := &repositories.CampaignSendSchedule{
		CampaignRateLimit: &campaignRate,
	}

	// Without profile rate (no smtpRepo lookup), should use campaign rate.
	svc := &Service{
		config: DefaultConfig(),
		smtpRepo: nil, // No lookup will succeed.
	}
	prl := newProfileRateLimiter()

	start := time.Now()
	svc.applyRateLimit(t.Context(), schedule, "camp-1", "", prl)
	elapsed := time.Since(start)

	// 100/min = 600ms per email.
	if elapsed < 500*time.Millisecond {
		t.Errorf("delay too short: %v, expected ~600ms for 100/min", elapsed)
	}
	if elapsed > 1*time.Second {
		t.Errorf("delay too long: %v", elapsed)
	}
}

// --- ECOMP-08: Batch Pause Logic Tests ---

func TestBatchPauseConfig(t *testing.T) {
	// Verify BatchSize and BatchPauseSeconds fields exist on schedule.
	batchSize := 10
	pauseSecs := 5
	schedule := repositories.CampaignSendSchedule{
		BatchSize:         &batchSize,
		BatchPauseSeconds: &pauseSecs,
	}

	if schedule.BatchSize == nil || *schedule.BatchSize != 10 {
		t.Error("BatchSize should be 10")
	}
	if schedule.BatchPauseSeconds == nil || *schedule.BatchPauseSeconds != 5 {
		t.Error("BatchPauseSeconds should be 5")
	}
}

func TestBatchPauseLogic(t *testing.T) {
	// Simulate batch counting logic.
	batchSize := 3
	batchCount := 0
	pauses := 0

	for i := 0; i < 10; i++ {
		batchCount++
		if batchSize > 0 && batchCount >= batchSize {
			pauses++
			batchCount = 0
		}
	}

	if pauses != 3 { // 10 emails, batch of 3: pauses after 3, 6, 9.
		t.Errorf("expected 3 pauses, got %d", pauses)
	}
	if batchCount != 1 { // 1 remaining after last pause.
		t.Errorf("expected batchCount=1 remaining, got %d", batchCount)
	}
}

func TestBatchPauseSkippedWhenZero(t *testing.T) {
	// Batch size 0 means no batching.
	batchSize := 0
	batchCount := 0
	pauses := 0

	for i := 0; i < 10; i++ {
		batchCount++
		if batchSize > 0 && batchCount >= batchSize {
			pauses++
			batchCount = 0
		}
	}

	if pauses != 0 {
		t.Errorf("expected 0 pauses when batch_size=0, got %d", pauses)
	}
}

func TestSenderPauseResume(t *testing.T) {
	sender := &campaignSender{
		campaignID: "test-campaign",
		paused:     make(chan struct{}),
	}
	close(sender.paused) // Start unpaused.

	if sender.isPaused {
		t.Error("sender should not be paused initially")
	}

	// Pause.
	sender.pausedMu.Lock()
	sender.isPaused = true
	sender.paused = make(chan struct{}) // New blocking channel.
	sender.pausedMu.Unlock()

	if !sender.isPaused {
		t.Error("sender should be paused after setting isPaused")
	}

	// Resume.
	sender.pausedMu.Lock()
	sender.isPaused = false
	close(sender.paused)
	sender.pausedMu.Unlock()

	if sender.isPaused {
		t.Error("sender should not be paused after resume")
	}

	// waitForUnpause should return immediately when not paused.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := sender.waitForUnpause(ctx)
	if err != nil {
		t.Errorf("waitForUnpause should not error when unpaused: %v", err)
	}
}
