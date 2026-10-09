package connmanager

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/coder/websocket"
)

var (
	ErrClientNotFound = errors.New("client session not connected")
)

// Manager coordinates client WebSocket connections and message routing.
// Users are identified EXCLUSIVELY via Session ID provided by the bot.
type Manager struct {
	clientsMu sync.RWMutex
	clients   map[string]*Client // sessionID -> Client
	handler   RouterHandler
}

// NewManager creates a new Connection Manager Agent.
func NewManager(handler RouterHandler) *Manager {
	return &Manager{
		clients: make(map[string]*Client),
		handler: handler,
	}
}

// SetHandler updates the Game Logic Agent handler for routing.
func (m *Manager) SetHandler(handler RouterHandler) {
	m.clientsMu.Lock()
	defer m.clientsMu.Unlock()
	m.handler = handler
}

// ServeHTTP handles the WebSocket handshake and initiates connection lifecycle.
func (m *Manager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session")
	userJID := r.URL.Query().Get("user")

	if sessionID == "" {
		http.Error(w, "missing session id: user must be authenticated via bot session", http.StatusUnauthorized)
		return
	}

	opts := &websocket.AcceptOptions{
		OriginPatterns: []string{"*"}, // Allow in-app WhatsApp webview / browser origins
	}

	conn, err := websocket.Accept(w, r, opts)
	if err != nil {
		log.Printf("[ConnManager] WebSocket accept error from %s: %v", r.RemoteAddr, err)
		return
	}

	client := newClient(r.Context(), sessionID, userJID, conn)

	m.register(client)
	go client.writePump()

	// Initial welcome message acknowledging session
	_ = m.SendToSession(sessionID, OutboundMessage{
		Type: "connected",
		Payload: map[string]string{
			"session_id": sessionID,
			"user_jid":   userJID,
		},
	})

	// Read loop pump
	m.readPump(client)
}

func (m *Manager) register(client *Client) {
	m.clientsMu.Lock()
	defer m.clientsMu.Unlock()

	// Disconnect existing client with same session ID if active
	if old, exists := m.clients[client.SessionID]; exists {
		old.cancel()
		_ = old.Conn.Close(websocket.StatusPolicyViolation, "superseded by new connection")
	}

	m.clients[client.SessionID] = client
	log.Printf("[ConnManager] Client connected: session=%s (total active: %d)", client.SessionID, len(m.clients))
}

func (m *Manager) unregister(sessionID string) {
	m.clientsMu.Lock()
	if c, ok := m.clients[sessionID]; ok {
		c.cancel()
		delete(m.clients, sessionID)
		log.Printf("[ConnManager] Client disconnected: session=%s (total active: %d)", sessionID, len(m.clients))
	}
	h := m.handler
	m.clientsMu.Unlock()

	if h != nil {
		h.HandleClientDisconnect(sessionID)
	}
}

func (m *Manager) readPump(c *Client) {
	defer func() {
		m.unregister(c.SessionID)
		_ = c.Conn.Close(websocket.StatusNormalClosure, "read loop ended")
	}()

	for {
		_, data, err := c.Conn.Read(c.ctx)
		if err != nil {
			return
		}

		var raw struct {
			Action  string          `json:"action"`
			Payload json.RawMessage `json:"payload"`
		}

		if err := json.Unmarshal(data, &raw); err != nil {
			_ = m.SendToSession(c.SessionID, OutboundMessage{
				Type:  "error",
				Error: "invalid json format",
			})
			continue
		}

		inbound := InboundMessage{
			SessionID: c.SessionID,
			Action:    raw.Action,
			Payload:   raw.Payload,
		}

		m.clientsMu.RLock()
		h := m.handler
		m.clientsMu.RUnlock()

		if h != nil {
			h.HandleInboundMessage(c.ctx, inbound)
		}
	}
}

// SendToSession delivers an outbound message to a specific user tracked by session ID.
func (m *Manager) SendToSession(sessionID string, msg OutboundMessage) error {
	m.clientsMu.RLock()
	client, exists := m.clients[sessionID]
	m.clientsMu.RUnlock()

	if !exists {
		return fmt.Errorf("%w: %s", ErrClientNotFound, sessionID)
	}

	bytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal outbound message: %w", err)
	}

	select {
	case client.SendChan <- bytes:
		return nil
	default:
		return fmt.Errorf("client send buffer full: %s", sessionID)
	}
}

// BroadcastToSessions delivers a message to a list of target session IDs.
func (m *Manager) BroadcastToSessions(sessionIDs []string, msg OutboundMessage) {
	for _, sid := range sessionIDs {
		_ = m.SendToSession(sid, msg)
	}
}

// IsConnected returns whether a session ID has an active WebSocket connection.
func (m *Manager) IsConnected(sessionID string) bool {
	m.clientsMu.RLock()
	defer m.clientsMu.RUnlock()
	_, exists := m.clients[sessionID]
	return exists
}

// ConnectedCount returns the count of active sessions.
func (m *Manager) ConnectedCount() int {
	m.clientsMu.RLock()
	defer m.clientsMu.RUnlock()
	return len(m.clients)
}
