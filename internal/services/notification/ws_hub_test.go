package notification

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// newTestServer spins up an httptest.Server with a minimal WebSocket echo that
// registers a client with the given hub.
func newTestServer(t *testing.T, hub *Hub, userID string) *httptest.Server {
	t.Helper()
	up := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		c := NewClient(userID, conn)
		hub.Register(c)
		go c.WritePump()
		c.ReadPump(hub)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// wsURL converts an http:// test server URL to ws://.
func wsURL(u string) string {
	return strings.Replace(u, "http://", "ws://", 1)
}

func TestHub_SendToUser_DeliversToRegisteredClient(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	const userID = "user-1"
	srv := newTestServer(t, hub, userID)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(srv.URL), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Give the hub a moment to process the register message.
	time.Sleep(50 * time.Millisecond)

	want := []byte(`{"type":"test","data":"hello"}`)
	hub.SendToUser(userID, want)

	conn.SetReadDeadline(time.Now().Add(2 * time.Second)) //nolint:errcheck
	_, got, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestHub_DisconnectedClients_AreCleanedUp(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	const userID = "user-2"
	srv := newTestServer(t, hub, userID)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(srv.URL), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Close the connection; ReadPump should unregister the client.
	conn.Close()
	time.Sleep(100 * time.Millisecond)

	hub.mu.RLock()
	_, exists := hub.clients[userID]
	hub.mu.RUnlock()

	if exists {
		t.Error("expected client to be removed from hub after disconnect")
	}
}

func TestHub_MaxConnsPerUser_EvictsOldest(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	const userID = "user-3"
	srv := newTestServer(t, hub, userID)

	// Open maxConnsPerUser+1 connections.
	conns := make([]*websocket.Conn, maxConnsPerUser+1)
	for i := range conns {
		c, _, err := websocket.DefaultDialer.Dial(wsURL(srv.URL), nil)
		if err != nil {
			t.Fatalf("dial %d: %v", i, err)
		}
		conns[i] = c
		time.Sleep(20 * time.Millisecond) // allow register to process
	}
	t.Cleanup(func() {
		for _, c := range conns {
			if c != nil {
				c.Close()
			}
		}
	})

	time.Sleep(100 * time.Millisecond)

	hub.mu.RLock()
	n := len(hub.clients[userID])
	hub.mu.RUnlock()

	if n > maxConnsPerUser {
		t.Errorf("got %d connections, want <= %d", n, maxConnsPerUser)
	}
}
