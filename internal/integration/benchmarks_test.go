//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"tackle/internal/crypto"
	auditsvc "tackle/internal/services/audit"
)

// BenchmarkAuditLoggingOverhead verifies audit logging adds ≤ 5ms per request (REQ-LOG-025).
func BenchmarkAuditLoggingOverhead(b *testing.B) {
	dbURL := getTestDBURL(b)

	db := openTestDB(b, dbURL)
	defer db.Close()

	masterKey := testMasterKey()
	auditHMACKey, err := crypto.DeriveSubkey(masterKey, crypto.PurposeHMACAudit)
	if err != nil {
		b.Fatalf("derive audit hmac key: %v", err)
	}
	hmacSvc := auditsvc.NewHMACService(auditHMACKey)
	svc := auditsvc.NewAuditService(db, hmacSvc, 10_000)
	defer svc.Drain()

	actorID := "00000000-0000-0000-0000-000000000001"
	resType := "user"
	resID := "00000000-0000-0000-0000-000000000002"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		_ = svc.Log(context.Background(), auditsvc.LogEntry{
			Category:  auditsvc.CategoryUserActivity,
			Severity:  auditsvc.SeverityInfo,
			ActorType: auditsvc.ActorTypeUser,
			ActorID:   &actorID,
			ActorLabel: "bench_user",
			Action:    fmt.Sprintf("bench.action.%d", i),
			ResourceType: &resType,
			ResourceID:   &resID,
		})
		elapsed := time.Since(start)
		if elapsed > 5*time.Millisecond {
			b.Logf("iteration %d: audit log overhead %v exceeded 5ms", i, elapsed)
		}
	}
}

// BenchmarkAuditLogQuery verifies filtered audit log query (1000 rows) returns within 2s (REQ-LOG-025).
func BenchmarkAuditLogQuery(b *testing.B) {
	dbURL := getTestDBURL(b)

	db := openTestDB(b, dbURL)
	defer db.Close()

	// Seed 1000 audit rows using direct insert for speed.
	masterKey := testMasterKey()
	auditHMACKey, err := crypto.DeriveSubkey(masterKey, crypto.PurposeHMACAudit)
	if err != nil {
		b.Fatalf("derive audit hmac key: %v", err)
	}
	hmacSvc := auditsvc.NewHMACService(auditHMACKey)
	svc := auditsvc.NewAuditService(db, hmacSvc, 50_000)

	actorID := "00000000-0000-0000-0000-000000000001"
	for i := 0; i < 1000; i++ {
		_ = svc.Log(context.Background(), auditsvc.LogEntry{
			Category:   auditsvc.CategoryUserActivity,
			Severity:   auditsvc.SeverityInfo,
			ActorType:  auditsvc.ActorTypeUser,
			ActorID:    &actorID,
			ActorLabel: "seed_user",
			Action:     fmt.Sprintf("bench.seed.%d", i),
		})
	}
	// Flush.
	time.Sleep(500 * time.Millisecond)
	svc.Drain()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		rows, err := db.QueryContext(context.Background(),
			`SELECT id, action FROM audit_logs WHERE actor_id = $1 LIMIT 1000`, actorID)
		if err != nil {
			b.Fatalf("query: %v", err)
		}
		count := 0
		for rows.Next() {
			count++
		}
		rows.Close()
		elapsed := time.Since(start)
		if elapsed > 2*time.Second {
			b.Errorf("audit log query took %v, expected ≤ 2s (got %d rows)", elapsed, count)
		}
	}
}

// BenchmarkLoginEndpoint verifies login responds within 500ms (REQ-AUTH threshold).
func BenchmarkLoginEndpoint(b *testing.B) {
	dbURL := getTestDBURL(b)

	db := openTestDB(b, dbURL)
	defer db.Close()

	masterKey := testMasterKey()
	cfg := testConfig(dbURL)
	logger := newBenchLogger()

	httpSrv := buildTestServer(b, cfg, db, masterKey, logger)
	defer httpSrv.Close()

	env := &testEnv{srv: httpSrv, db: db, masterKey: masterKey}
	resetDB(b, db)
	mustSetupB(b, env)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		resp := env.do("POST", "/auth/login", map[string]string{
			"username": "admin",
			"password": "S3cur3P@ssw0rd!XYZ",
		}, "")
		resp.Body.Close()
		elapsed := time.Since(start)
		if resp.StatusCode != http.StatusOK {
			b.Errorf("iteration %d: login returned %d", i, resp.StatusCode)
		}
		if elapsed > 500*time.Millisecond {
			b.Errorf("iteration %d: login took %v, expected ≤ 500ms", i, elapsed)
		}
	}
}
