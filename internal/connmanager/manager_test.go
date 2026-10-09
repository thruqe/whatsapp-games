package connmanager

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type mockHandler struct {
	inboundChan    chan InboundMessage
	disconnectChan chan string
}

func (m *mockHandler) HandleInboundMessage(ctx context.Context, msg InboundMessage) {
	m.inboundChan <- msg
}

func (m *mockHandler) HandleClientDisconnect(sessionID string) {
	m.disconnectChan <- sessionID
}

func TestConnectionManager(t *testing.T) {
	handler := &mockHandler{
		inboundChan:    make(chan InboundMessage, 10),
		disconnectChan: make(chan string, 10),
	}

	mgr := NewManager(handler)
	server := httptest.NewServer(mgr)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?session=sess_abc&user=123@s.whatsapp.net"

	// Connect client
	c, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}

	// Should receive "connected" welcome message
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read connected message: %v", err)
	}

	var welcome OutboundMessage
	if err := json.Unmarshal(data, &welcome); err != nil || welcome.Type != "connected" {
		t.Fatalf("expected connected message, got: %s", string(data))
	}

	if !mgr.IsConnected("sess_abc") {
		t.Fatalf("expected sess_abc to be marked as connected")
	}

	// Send an inbound action
	testPayload := json.RawMessage(`{"name":"test"}`)
	outbound := map[string]any{
		"action":  "ping",
		"payload": testPayload,
	}
	sendBytes, _ := json.Marshal(outbound)
	if err := c.Write(ctx, websocket.MessageText, sendBytes); err != nil {
		t.Fatalf("failed to write message: %v", err)
	}

	// Check if router routed message to handler
	select {
	case msg := <-handler.inboundChan:
		if msg.SessionID != "sess_abc" || msg.Action != "ping" {
			t.Fatalf("unexpected inbound message: %+v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for routed inbound message")
	}

	// Test SendToSession from manager
	err = mgr.SendToSession("sess_abc", OutboundMessage{
		Type: "pong",
	})
	if err != nil {
		t.Fatalf("failed to send to session: %v", err)
	}

	_, pongData, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read pong: %v", err)
	}
	var pong OutboundMessage
	if err := json.Unmarshal(pongData, &pong); err != nil || pong.Type != "pong" {
		t.Fatalf("expected pong message, got: %s", string(pongData))
	}

	// Disconnect client
	_ = c.Close(websocket.StatusNormalClosure, "done")

	select {
	case discSess := <-handler.disconnectChan:
		if discSess != "sess_abc" {
			t.Fatalf("expected disconnect for sess_abc, got: %s", discSess)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for disconnect notice")
	}

	if mgr.IsConnected("sess_abc") {
		t.Fatalf("expected sess_abc to no longer be connected")
	}
}
