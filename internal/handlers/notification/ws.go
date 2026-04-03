package notification

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	notifsvc "tackle/internal/services/notification"
)

const (
	// authTimeout is how long a new connection has to send a valid auth message.
	authTimeout = 5 * time.Second
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins — the server is behind an internal network.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// wsAuthMessage is the expected first message from the client after connection.
type wsAuthMessage struct {
	Type  string `json:"type"`
	Token string `json:"token"`
}

// WS handles GET /api/v1/ws — upgrades the connection, then waits for a
// first-message auth: {"type":"auth","token":"<JWT>"}. The connection is
// closed if no valid auth message arrives within 5 seconds. Tokens are
// never passed in query parameters.
func (d *Deps) WS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrader writes the error response; just return.
		return
	}

	// Wait for auth message within the timeout window.
	_ = conn.SetReadDeadline(time.Now().Add(authTimeout))

	_, msg, err := conn.ReadMessage()
	if err != nil {
		_ = conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "auth timeout"))
		conn.Close()
		return
	}

	var authMsg wsAuthMessage
	if err := json.Unmarshal(msg, &authMsg); err != nil || authMsg.Type != "auth" || authMsg.Token == "" {
		_ = conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "invalid auth message"))
		conn.Close()
		return
	}

	claims, err := d.JWTSvc.Validate(authMsg.Token)
	if err != nil {
		_ = conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "invalid token"))
		conn.Close()
		return
	}

	// Auth successful — send confirmation and reset deadline.
	_ = conn.SetReadDeadline(time.Time{}) // clear deadline
	_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"auth_ok"}`))

	client := notifsvc.NewClient(claims.Subject, conn)
	d.Hub.Register(client)

	// Run pumps. WritePump owns the connection; ReadPump drives close detection.
	go client.WritePump()
	client.ReadPump(d.Hub) // blocks until closed
}
