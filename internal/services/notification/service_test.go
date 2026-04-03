package notification

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCreate_ResolvesRoleRecipients(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	hub := NewHub()
	go hub.Run()

	svc := NewNotificationService(db, hub)

	// Role query returns one user.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT DISTINCT ur.user_id`)).
		WithArgs("operator").
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow("user-abc"))

	// INSERT for that user.
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO notifications`)).
		WithArgs(
			sqlmock.AnyArg(), // id
			"user-abc",
			"system",
			"info",
			"Test title",
			"Test body",
			sqlmock.AnyArg(), // resource_type
			sqlmock.AnyArg(), // resource_id
			sqlmock.AnyArg(), // action_url
			sqlmock.AnyArg(), // expires_at
			sqlmock.AnyArg(), // created_at
		).WillReturnResult(sqlmock.NewResult(1, 1))

	svc.Create(context.Background(), CreateNotificationParams{
		Category:   "system",
		Severity:   "info",
		Title:      "Test title",
		Body:       "Test body",
		Recipients: RecipientSpec{Role: "operator"},
	})

	// Give the goroutine time to execute.
	time.Sleep(200 * time.Millisecond)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestCreate_ExplicitUserIDs(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	hub := NewHub()
	go hub.Run()

	svc := NewNotificationService(db, hub)

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO notifications`)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	svc.Create(context.Background(), CreateNotificationParams{
		Category:   "campaign_lifecycle",
		Severity:   "warning",
		Title:      "Campaign started",
		Body:       "Campaign ID referenced by id only",
		Recipients: RecipientSpec{UserIDs: []string{"user-1"}},
	})

	time.Sleep(200 * time.Millisecond)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestCreate_SendsToHub(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	hub := NewHub()
	go hub.Run()

	svc := NewNotificationService(db, hub)

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO notifications`)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Register a test client for user-1 to capture the push.
	received := make(chan []byte, 1)
	dummyClient := &Client{
		userID: "user-1",
		send:   make(chan []byte, sendBufSize),
	}
	hub.Register(dummyClient)
	time.Sleep(50 * time.Millisecond)

	go func() {
		msg := <-dummyClient.send
		received <- msg
	}()

	svc.Create(context.Background(), CreateNotificationParams{
		Category:   "system",
		Severity:   "info",
		Title:      "Hub test",
		Body:       "body",
		Recipients: RecipientSpec{UserIDs: []string{"user-1"}},
	})

	select {
	case msg := <-received:
		if len(msg) == 0 {
			t.Error("expected non-empty WebSocket message")
		}
	case <-time.After(2 * time.Second):
		t.Error("timed out waiting for WebSocket message")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
