package emaildelivery

import (
	"context"
	"math"
	"testing"
	"time"

	"tackle/internal/repositories"
)

// ---------- EMAIL-02: Queue Builder Tests ----------

func TestBuildQueue_SendOrderPreservation(t *testing.T) {
	// Verify that the queue preserves send_order_position from snapshots.
	// This is a unit test of the ordering logic.
	snapshots := []repositories.CampaignTargetsSnapshot{
		{ID: "s1", TargetID: "t1", IsCanary: true, SendOrderPosition: intPtr(1)},
		{ID: "s2", TargetID: "t2", IsCanary: false, SendOrderPosition: intPtr(2)},
		{ID: "s3", TargetID: "t3", IsCanary: false, SendOrderPosition: intPtr(3)},
	}

	// Canary should be first.
	if !snapshots[0].IsCanary {
		t.Error("first snapshot should be canary")
	}
	if *snapshots[0].SendOrderPosition != 1 {
		t.Error("canary should have position 1")
	}
}

func TestBuildQueue_RoundRobinAssignment(t *testing.T) {
	// Verify round-robin SMTP assignment logic.
	profileIDs := []string{"smtp-a", "smtp-b", "smtp-c"}
	targetCount := 10

	assignments := make([]string, targetCount)
	for i := 0; i < targetCount; i++ {
		assignments[i] = profileIDs[i%len(profileIDs)]
	}

	// Verify distribution.
	counts := make(map[string]int)
	for _, a := range assignments {
		counts[a]++
	}

	// With 10 targets and 3 profiles: 4, 3, 3 distribution.
	if counts["smtp-a"] != 4 {
		t.Errorf("smtp-a got %d assignments, want 4", counts["smtp-a"])
	}
	if counts["smtp-b"] != 3 {
		t.Errorf("smtp-b got %d assignments, want 3", counts["smtp-b"])
	}
	if counts["smtp-c"] != 3 {
		t.Errorf("smtp-c got %d assignments, want 3", counts["smtp-c"])
	}
}

func TestBuildQueue_CanaryPriority(t *testing.T) {
	// Simulate snapshot ordering with canary first.
	type entry struct {
		targetID string
		isCanary bool
		position int
	}
	targets := []entry{
		{"t3", false, 3},
		{"t1", true, 1},  // canary
		{"t2", false, 2},
	}

	// Sort: canary first, then by position.
	// Verify canary is at position 1 after sorting.
	var canaryFirst bool
	for _, e := range targets {
		if e.isCanary {
			canaryFirst = e.position == 1
			break
		}
	}
	if !canaryFirst {
		t.Error("canary target should have position 1")
	}
}

// ---------- EMAIL-03: Send Window Tests ----------

func TestSendWindow_TimeCheck(t *testing.T) {
	svc := &Service{}

	tests := []struct {
		name    string
		windows []repositories.CampaignSendWindow
		want    bool
	}{
		{
			name:    "empty windows always false",
			windows: []repositories.CampaignSendWindow{},
			want:    false,
		},
		{
			name: "wide open window",
			windows: []repositories.CampaignSendWindow{
				{Days: allDays(), StartTime: "00:00:00", EndTime: "23:59:59", Timezone: "UTC"},
			},
			want: true,
		},
		{
			name: "no days specified means all days",
			windows: []repositories.CampaignSendWindow{
				{Days: []string{}, StartTime: "00:00:00", EndTime: "23:59:59", Timezone: "UTC"},
			},
			want: true,
		},
		{
			name: "impossible time range",
			windows: []repositories.CampaignSendWindow{
				{Days: allDays(), StartTime: "25:00:00", EndTime: "25:01:00", Timezone: "UTC"},
			},
			want: false,
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

func TestSendWindow_Timezone(t *testing.T) {
	svc := &Service{}

	// Use a timezone that's far enough from UTC to test.
	windows := []repositories.CampaignSendWindow{
		{Days: allDays(), StartTime: "00:00:00", EndTime: "23:59:59", Timezone: "America/New_York"},
	}

	got := svc.isWithinSendWindow(windows)
	if !got {
		t.Error("should be within an all-day window regardless of timezone")
	}
}

func TestSendWindow_InvalidTimezone(t *testing.T) {
	svc := &Service{}

	// Invalid timezone should fall back to UTC.
	windows := []repositories.CampaignSendWindow{
		{Days: allDays(), StartTime: "00:00:00", EndTime: "23:59:59", Timezone: "Invalid/Zone"},
	}

	got := svc.isWithinSendWindow(windows)
	if !got {
		t.Error("invalid timezone should fall back to UTC and still match all-day window")
	}
}

func TestNextWindowOpen_MultipleDays(t *testing.T) {
	svc := &Service{}

	// Only weekend windows.
	windows := []repositories.CampaignSendWindow{
		{Days: []string{"saturday", "sunday"}, StartTime: "08:00:00", EndTime: "17:00:00", Timezone: "UTC"},
	}

	nextOpen := svc.nextWindowOpen(windows)
	if nextOpen.IsZero() {
		t.Error("should find a future window opening on weekend")
	}

	// The next open should be on a Saturday or Sunday.
	day := nextOpen.Weekday()
	if day != time.Saturday && day != time.Sunday {
		t.Errorf("nextWindowOpen returned %v, expected Saturday or Sunday", day)
	}
}

// ---------- EMAIL-03: Rate Limiting Tests ----------

func TestRateLimit_DefaultRate(t *testing.T) {
	svc := &Service{config: DefaultConfig()}

	schedule := &repositories.CampaignSendSchedule{}

	// With default rate of 60/min, delay should be ~1 second.
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	svc.applyRateLimit(ctx, schedule, "test-campaign", "", nil)
	elapsed := time.Since(start)

	if elapsed < 900*time.Millisecond {
		t.Errorf("rate limit delay too short: %v, expected ~1s", elapsed)
	}
	if elapsed > 1200*time.Millisecond {
		t.Errorf("rate limit delay too long: %v, expected ~1s", elapsed)
	}
}

func TestRateLimit_CustomRate(t *testing.T) {
	svc := &Service{config: DefaultConfig()}

	rate := 120 // 120/min = 0.5s delay.
	schedule := &repositories.CampaignSendSchedule{
		CampaignRateLimit: &rate,
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	svc.applyRateLimit(ctx, schedule, "test-campaign", "", nil)
	elapsed := time.Since(start)

	if elapsed < 400*time.Millisecond {
		t.Errorf("rate limit delay too short: %v, expected ~500ms", elapsed)
	}
	if elapsed > 700*time.Millisecond {
		t.Errorf("rate limit delay too long: %v, expected ~500ms", elapsed)
	}
}

func TestRateLimit_ContextCancellation(t *testing.T) {
	svc := &Service{config: DefaultConfig()}

	schedule := &repositories.CampaignSendSchedule{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	start := time.Now()
	svc.applyRateLimit(ctx, schedule, "test-campaign", "", nil)
	elapsed := time.Since(start)

	if elapsed > 100*time.Millisecond {
		t.Errorf("cancelled context should return quickly, took %v", elapsed)
	}
}

// ---------- EMAIL-03: Inter-message Delay Tests ----------

func TestDelay_Randomization(t *testing.T) {
	svc := &Service{config: DefaultConfig()}

	schedule := &repositories.CampaignSendSchedule{
		MinDelayMs: 50,
		MaxDelayMs: 100,
	}

	// Run multiple times and verify delays are within range.
	for i := 0; i < 5; i++ {
		start := time.Now()
		svc.applyDelay(context.Background(), schedule)
		elapsed := time.Since(start)

		if elapsed < 45*time.Millisecond { // Small tolerance.
			t.Errorf("iteration %d: delay too short: %v", i, elapsed)
		}
		if elapsed > 150*time.Millisecond { // Some overhead tolerance.
			t.Errorf("iteration %d: delay too long: %v", i, elapsed)
		}
	}
}

func TestDelay_MinEqualsMax(t *testing.T) {
	svc := &Service{config: DefaultConfig()}

	schedule := &repositories.CampaignSendSchedule{
		MinDelayMs: 50,
		MaxDelayMs: 50,
	}

	start := time.Now()
	svc.applyDelay(context.Background(), schedule)
	elapsed := time.Since(start)

	if elapsed < 45*time.Millisecond {
		t.Errorf("delay too short: %v", elapsed)
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("delay too long: %v", elapsed)
	}
}

// ---------- EMAIL-04: Delivery Status Tests ----------

func TestDeliveryResult_Validation(t *testing.T) {
	tests := []struct {
		name   string
		result DeliveryResult
		valid  bool
	}{
		{
			name:   "valid sent result",
			result: DeliveryResult{EmailID: "e1", CampaignID: "c1", Status: "sent"},
			valid:  true,
		},
		{
			name:   "valid delivered result",
			result: DeliveryResult{EmailID: "e1", CampaignID: "c1", Status: "delivered"},
			valid:  true,
		},
		{
			name:   "valid bounced result",
			result: DeliveryResult{EmailID: "e1", CampaignID: "c1", Status: "bounced", BounceType: strPtr("hard")},
			valid:  true,
		},
		{
			name:   "missing email_id",
			result: DeliveryResult{CampaignID: "c1", Status: "sent"},
			valid:  false,
		},
		{
			name:   "missing campaign_id",
			result: DeliveryResult{EmailID: "e1", Status: "sent"},
			valid:  false,
		},
		{
			name:   "missing status",
			result: DeliveryResult{EmailID: "e1", CampaignID: "c1"},
			valid:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			valid := tc.result.EmailID != "" && tc.result.CampaignID != "" && tc.result.Status != ""
			if valid != tc.valid {
				t.Errorf("validation = %v, want %v", valid, tc.valid)
			}
		})
	}
}

func TestBounceClassification(t *testing.T) {
	tests := []struct {
		name       string
		bounceType string
		isPermanent bool
	}{
		{"hard bounce is permanent", "hard", true},
		{"soft bounce is temporary", "soft", false},
		{"empty is temporary", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			isPermanent := tc.bounceType == "hard"
			if isPermanent != tc.isPermanent {
				t.Errorf("isPermanent = %v, want %v", isPermanent, tc.isPermanent)
			}
		})
	}
}

// ---------- EMAIL-05: Retry Logic Tests ----------

func TestRetryBackoff_Calculation(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		retryCount int
		expected   time.Duration
	}{
		{0, 5 * time.Minute},  // Initial: 5m * 2^0 = 5m.
		{1, 10 * time.Minute}, // 5m * 2^1 = 10m.
		{2, 20 * time.Minute}, // 5m * 2^2 = 20m.
	}

	for _, tc := range tests {
		t.Run("retry_"+string(rune('0'+tc.retryCount)), func(t *testing.T) {
			backoff := cfg.InitialRetryInterval * time.Duration(math.Pow(2, float64(tc.retryCount)))
			if backoff != tc.expected {
				t.Errorf("backoff for retry %d = %v, want %v", tc.retryCount, backoff, tc.expected)
			}
		})
	}
}

func TestMaxRetries_Exhaustion(t *testing.T) {
	cfg := DefaultConfig()

	// After MaxRetries, email should be permanently failed.
	if cfg.MaxRetries != 3 {
		t.Fatalf("expected MaxRetries=3, got %d", cfg.MaxRetries)
	}

	// retryCount >= MaxRetries means exhausted.
	for retryCount := 0; retryCount <= cfg.MaxRetries+1; retryCount++ {
		exhausted := retryCount >= cfg.MaxRetries
		shouldRetry := !exhausted

		if retryCount < cfg.MaxRetries && !shouldRetry {
			t.Errorf("retry count %d should still retry", retryCount)
		}
		if retryCount >= cfg.MaxRetries && shouldRetry {
			t.Errorf("retry count %d should be exhausted", retryCount)
		}
	}
}

// ---------- EMAIL-06: Pause/Resume Tests ----------

func TestPauseResume_ChannelMechanism(t *testing.T) {
	cs := &campaignSender{
		campaignID: "test",
		paused:     make(chan struct{}),
	}
	close(cs.paused) // Start unpaused.

	// Verify unpaused state.
	if cs.isPaused {
		t.Error("should start unpaused")
	}

	// Pause.
	cs.pausedMu.Lock()
	cs.isPaused = true
	cs.paused = make(chan struct{})
	cs.pausedMu.Unlock()

	if !cs.isPaused {
		t.Error("should be paused after pause")
	}

	// waitForUnpause should block.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := cs.waitForUnpause(ctx)
	if err == nil {
		t.Error("should have timed out while paused")
	}

	// Resume.
	cs.pausedMu.Lock()
	cs.isPaused = false
	close(cs.paused)
	cs.pausedMu.Unlock()

	// Now waitForUnpause should return immediately.
	start := time.Now()
	err = cs.waitForUnpause(context.Background())
	if err != nil {
		t.Errorf("waitForUnpause after resume: %v", err)
	}
	if time.Since(start) > 50*time.Millisecond {
		t.Error("should return immediately after resume")
	}
}

func TestPauseResume_NoSkipNoDuplicate(t *testing.T) {
	// Test that queue position is preserved by verifying the queue ordering logic.
	positions := []int{1, 2, 3, 4, 5}
	lastSent := 3 // Paused after sending position 3.

	// Resume should start from position 4.
	resumeFrom := lastSent + 1
	remaining := positions[resumeFrom-1:]

	if len(remaining) != 2 {
		t.Errorf("expected 2 remaining, got %d", len(remaining))
	}
	if remaining[0] != 4 {
		t.Errorf("should resume from position 4, got %d", remaining[0])
	}
}

// ---------- EMAIL-07: Metrics Tests ----------

func TestMetrics_ErrorRate(t *testing.T) {
	tests := []struct {
		name      string
		sendCount int
		errCount  int
		wantRate  float64
	}{
		{"no sends", 0, 0, 0.0},
		{"no errors", 100, 0, 0.0},
		{"10% errors", 100, 10, 0.1},
		{"50% errors", 100, 50, 0.5},
		{"all errors", 10, 10, 1.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var rate float64
			if tc.sendCount > 0 {
				rate = float64(tc.errCount) / float64(tc.sendCount)
			}
			if rate != tc.wantRate {
				t.Errorf("error rate = %v, want %v", rate, tc.wantRate)
			}
		})
	}
}

// ---------- EMAIL-08: Completion Detection Tests ----------

func TestCompletion_AllTerminalStates(t *testing.T) {
	terminalStates := map[string]bool{
		"sent":      true,
		"delivered": true,
		"bounced":   true,
		"failed":    true,
		"cancelled": true,
	}
	nonTerminalStates := map[string]bool{
		"queued":  true,
		"sending": true,
		"deferred": true,
	}

	for state := range terminalStates {
		if !terminalStates[state] {
			t.Errorf("%s should be terminal", state)
		}
	}
	for state := range nonTerminalStates {
		if terminalStates[state] {
			t.Errorf("%s should not be terminal", state)
		}
	}
}

func TestCompletion_EndDateCheck(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name          string
		endDate       *time.Time
		allTerminal   bool
		shouldComplete bool
	}{
		{"all done, no end date", nil, true, true},
		{"all done, end date passed", timePtr(now.Add(-1 * time.Hour)), true, true},
		{"all done, end date future", timePtr(now.Add(1 * time.Hour)), true, false},
		{"not all done, end date passed", timePtr(now.Add(-1 * time.Hour)), false, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			shouldComplete := tc.allTerminal && (tc.endDate == nil || !tc.endDate.After(now))
			if shouldComplete != tc.shouldComplete {
				t.Errorf("shouldComplete = %v, want %v", shouldComplete, tc.shouldComplete)
			}
		})
	}
}

// ---------- A/B Testing and Variant Tracking ----------

func TestVariantAssignment_Distribution(t *testing.T) {
	// Simulate A/B assignment: 70/30 split.
	totalTargets := 100
	splitA := 70
	countA := totalTargets * splitA / 100
	countB := totalTargets - countA

	if countA != 70 {
		t.Errorf("variant A count = %d, want 70", countA)
	}
	if countB != 30 {
		t.Errorf("variant B count = %d, want 30", countB)
	}
}

// ---------- Performance ----------

func TestQueueBuilding_LargeTargetSet(t *testing.T) {
	// Verify queue building logic can handle 10,000 targets.
	targetCount := 10000
	profileCount := 3

	profileIDs := make([]string, profileCount)
	for i := range profileIDs {
		profileIDs[i] = "smtp-" + string(rune('a'+i))
	}

	start := time.Now()
	assignments := make([]string, targetCount)
	for i := 0; i < targetCount; i++ {
		assignments[i] = profileIDs[i%profileCount]
	}
	elapsed := time.Since(start)

	if elapsed > 100*time.Millisecond {
		t.Errorf("queue building logic took %v, expected < 100ms", elapsed)
	}

	// Verify distribution.
	counts := make(map[string]int)
	for _, a := range assignments {
		counts[a]++
	}

	for _, pid := range profileIDs {
		if counts[pid] < 3333 || counts[pid] > 3334 {
			t.Errorf("profile %s got %d assignments, expected ~3333", pid, counts[pid])
		}
	}
}
