package connmanager

import (
	"context"
	"encoding/json"
	"time"

	"github.com/coder/websocket"
)

// InboundMessage represents a parsed message received from a client WebSocket.
type InboundMessage struct {
	SessionID string          `json:"session_id"`
	Action    string          `json:"action"`
	Payload   json.RawMessage `json:"payload"`
}

// OutboundMessage represents a payload sent to a client WebSocket.
type OutboundMessage struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
	Error   string `json:"error,omitempty"`
}

// RouterHandler defines the callback interface for Game Logic Agent to receive messages.
type RouterHandler interface {
	HandleInboundMessage(ctx context.Context, msg InboundMessage)
	HandleClientDisconnect(sessionID string)
}

// Client represents a connected user identified exclusively by SessionID.
type Client struct {
	SessionID string
	UserJID   string
	Conn      *websocket.Conn
	SendChan  chan []byte
	ctx       context.Context
	cancel    context.CancelFunc
}

func newClient(ctx context.Context, sessionID, userJID string, conn *websocket.Conn) *Client {
	cCtx, cancel := context.WithCancel(ctx)
	return &Client{
		SessionID: sessionID,
		UserJID:   userJID,
		Conn:      conn,
		SendChan:  make(chan []byte, 64),
		ctx:       cCtx,
		cancel:    cancel,
	}
}

func (c *Client) writePump() {
	defer func() {
		_ = c.Conn.Close(websocket.StatusNormalClosure, "closing write pump")
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		case data, ok := <-c.SendChan:
			if !ok {
				return
			}
			ctx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
			err := c.Conn.Write(ctx, websocket.MessageText, data)
			cancel()
			if err != nil {
				return
			}
		}
	}
}
