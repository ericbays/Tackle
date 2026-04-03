// Package notification provides the notification service, WebSocket hub, and related types.
package notification

import (
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	maxConnsPerUser   = 5
	pingInterval      = 30 * time.Second
	pongDeadline      = 10 * time.Second
	writeDeadline     = 10 * time.Second
	sendBufSize       = 64
)

// Client represents a single WebSocket connection for a user.
type Client struct {
	userID string
	conn   *websocket.Conn
	send   chan []byte
}

// Hub manages all active WebSocket connections, keyed by user ID.
// All mutations are serialised through the register/unregister/broadcast channels.
type Hub struct {
	// mu protects clients map reads outside the run loop (e.g. len checks).
	mu         sync.RWMutex
	clients    map[string][]*Client // userID → slice of connections

	register   chan *Client
	unregister chan *Client
	send       chan userMsg
	broadcast  chan []byte // messages sent to ALL connected clients
	done       chan struct{}
}

// userMsg is an internal message targeting a specific user.
type userMsg struct {
	userID  string
	payload []byte
}

// NewHub creates and returns an idle Hub. Call Run() in a goroutine to start it.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string][]*Client),
		register:   make(chan *Client, 16),
		unregister: make(chan *Client, 16),
		send:       make(chan userMsg, 256),
		broadcast:  make(chan []byte, 256),
		done:       make(chan struct{}),
	}
}

// Run processes hub events. Must be called exactly once in a dedicated goroutine.
func (h *Hub) Run() {
	defer close(h.done)
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			conns := h.clients[c.userID]
			if len(conns) >= maxConnsPerUser {
				// Evict the oldest connection.
				oldest := conns[0]
				conns = conns[1:]
				close(oldest.send)
			}
			h.clients[c.userID] = append(conns, c)
			h.mu.Unlock()

		case c := <-h.unregister:
			h.mu.Lock()
			conns := h.clients[c.userID]
			newConns := conns[:0]
			for _, existing := range conns {
				if existing != c {
					newConns = append(newConns, existing)
				}
			}
			if len(newConns) == 0 {
				delete(h.clients, c.userID)
			} else {
				h.clients[c.userID] = newConns
			}
			h.mu.Unlock()

		case msg := <-h.send:
			h.mu.RLock()
			conns := h.clients[msg.userID]
			h.mu.RUnlock()
			for _, c := range conns {
				select {
				case c.send <- msg.payload:
				default:
					// Send channel full — drop message rather than block.
					slog.Warn("ws_hub: send channel full, dropping message", "user_id", msg.userID)
				}
			}

		case msg := <-h.broadcast:
			h.mu.RLock()
			for _, conns := range h.clients {
				for _, c := range conns {
					select {
					case c.send <- msg:
					default:
						slog.Warn("ws_hub: broadcast send channel full, dropping message", "user_id", c.userID)
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Register adds a client to the hub. Non-blocking.
func (h *Hub) Register(c *Client) {
	h.register <- c
}

// Unregister removes a client from the hub. Non-blocking.
func (h *Hub) Unregister(c *Client) {
	h.unregister <- c
}

// SendToUser enqueues a message to all connections for the given user. Non-blocking.
func (h *Hub) SendToUser(userID string, msg []byte) {
	h.send <- userMsg{userID: userID, payload: msg}
}

// BroadcastAll enqueues a message to all connected clients regardless of user. Non-blocking.
func (h *Hub) BroadcastAll(msg []byte) {
	select {
	case h.broadcast <- msg:
	default:
		slog.Warn("ws_hub: broadcast channel full, dropping message")
	}
}

// NewClient creates a Client wrapping the given websocket connection.
func NewClient(userID string, conn *websocket.Conn) *Client {
	return &Client{
		userID: userID,
		conn:   conn,
		send:   make(chan []byte, sendBufSize),
	}
}

// WritePump drains the client's send channel and writes messages to the WebSocket.
// It also runs a periodic ping. Returns when the send channel is closed or a write fails.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeDeadline))
			if !ok {
				// Hub closed the channel.
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeDeadline))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ReadPump reads from the WebSocket connection until it closes.
// It sets up the pong handler which resets the read deadline on each pong received.
// Returns when the connection is closed or errors.
func (c *Client) ReadPump(hub *Hub) {
	defer func() {
		hub.Unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(pingInterval + pongDeadline)) //nolint:errcheck
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pingInterval + pongDeadline)) //nolint:errcheck
		return nil
	})

	for {
		// We only read to detect close frames; clients don't send application messages.
		if _, _, err := c.conn.ReadMessage(); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Debug("ws_hub: unexpected close", "user_id", c.userID, "error", err)
			}
			return
		}
	}
}
