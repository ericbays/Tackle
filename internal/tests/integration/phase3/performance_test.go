//go:build integration

package phase3

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// TestTargetListPerformance verifies target list pagination returns within 500ms
// for a large dataset. This test creates 1000 targets (scaled down from 100K for CI speed).
func TestTargetListPerformance(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)

	// Get admin user ID for FK.
	var adminUserID string
	if err := env.db.QueryRowContext(context.Background(),
		`SELECT id FROM users WHERE username = 'admin' LIMIT 1`).Scan(&adminUserID); err != nil {
		t.Fatalf("get admin user id: %v", err)
	}

	// Insert targets in batch via direct SQL for speed.
	const batchSize = 1000
	for i := 0; i < batchSize; i++ {
		_, err := env.db.ExecContext(context.Background(),
			`INSERT INTO targets (id, email, first_name, last_name, department, created_by, created_at, updated_at)
			 VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, NOW(), NOW())`,
			fmt.Sprintf("perf%d@example.com", i),
			fmt.Sprintf("First%d", i),
			fmt.Sprintf("Last%d", i),
			fmt.Sprintf("Dept%d", i%10),
			adminUserID,
		)
		if err != nil {
			t.Fatalf("insert target %d: %v", i, err)
		}
	}

	// Time the paginated list query.
	start := time.Now()
	resp := env.do("GET", "/targets?per_page=25&page=1", nil, adminToken)
	elapsed := time.Since(start)
	assertStatus(t, resp, http.StatusOK)
	var listResp struct {
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}
	decodeT(t, resp, &listResp)

	if listResp.Pagination.Total < batchSize {
		t.Errorf("expected total >= %d, got %d", batchSize, listResp.Pagination.Total)
	}

	// Performance gate: 500ms.
	if elapsed > 500*time.Millisecond {
		t.Errorf("target list took %v (limit 500ms)", elapsed)
	}
	t.Logf("target list (%d rows): %v", listResp.Pagination.Total, elapsed)
}

// TestTargetFilterPerformance verifies filtered search returns within 500ms.
func TestTargetFilterPerformance(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)

	// Get admin user ID for FK.
	var adminUID2 string
	if err := env.db.QueryRowContext(context.Background(),
		`SELECT id FROM users WHERE username = 'admin' LIMIT 1`).Scan(&adminUID2); err != nil {
		t.Fatalf("get admin user id: %v", err)
	}

	// Insert targets.
	const batchSize = 1000
	for i := 0; i < batchSize; i++ {
		dept := "Engineering"
		if i%3 == 0 {
			dept = "Sales"
		}
		_, err := env.db.ExecContext(context.Background(),
			`INSERT INTO targets (id, email, first_name, last_name, department, created_by, created_at, updated_at)
			 VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, NOW(), NOW())`,
			fmt.Sprintf("filter%d@example.com", i),
			fmt.Sprintf("F%d", i), fmt.Sprintf("L%d", i), dept, adminUID2,
		)
		if err != nil {
			t.Fatalf("insert target %d: %v", i, err)
		}
	}

	// Time a filtered query.
	start := time.Now()
	resp := env.do("GET", "/targets?department=Sales&per_page=25", nil, adminToken)
	elapsed := time.Since(start)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	if elapsed > 500*time.Millisecond {
		t.Errorf("filtered target search took %v (limit 500ms)", elapsed)
	}
	t.Logf("filtered target search (1K rows): %v", elapsed)
}

// TestCampaignListPerformance verifies campaign list returns within reasonable time.
func TestCampaignListPerformance(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)

	// Get admin user ID for FK.
	var adminUID string
	if err := env.db.QueryRowContext(context.Background(),
		`SELECT id FROM users WHERE username = 'admin' LIMIT 1`).Scan(&adminUID); err != nil {
		t.Fatalf("get admin user id: %v", err)
	}

	// Insert campaigns.
	const batchSize = 100
	for i := 0; i < batchSize; i++ {
		_, err := env.db.ExecContext(context.Background(),
			`INSERT INTO campaigns (id, name, description, current_state, send_order, created_by, created_at, updated_at)
			 VALUES (gen_random_uuid(), $1, 'Test campaign', 'draft', 'default', $2, NOW(), NOW())`,
			fmt.Sprintf("Perf Campaign %d", i), adminUID,
		)
		if err != nil {
			t.Fatalf("insert campaign %d: %v", i, err)
		}
	}

	start := time.Now()
	resp := env.do("GET", "/campaigns?per_page=25", nil, adminToken)
	elapsed := time.Since(start)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	if elapsed > 500*time.Millisecond {
		t.Errorf("campaign list took %v (limit 500ms)", elapsed)
	}
	t.Logf("campaign list (%d rows): %v", batchSize, elapsed)
}
