package workers

import (
	"log/slog"
	"testing"
	"time"
)

func TestNewDomainExpiryWorkerDefaults(t *testing.T) {
	w := NewDomainExpiryWorker(nil, 0, slog.Default())
	if w.interval != 24*time.Hour {
		t.Errorf("expected default interval 24h, got %v", w.interval)
	}
}

func TestNewDomainExpiryWorkerCustomInterval(t *testing.T) {
	w := NewDomainExpiryWorker(nil, 12*time.Hour, slog.Default())
	if w.interval != 12*time.Hour {
		t.Errorf("expected interval 12h, got %v", w.interval)
	}
}
