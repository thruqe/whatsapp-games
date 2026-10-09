package gamelogic

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// PlayerInfo stores session and display metadata for a matched player.
type PlayerInfo struct {
	SessionID  string `json:"session_id"`
	Username   string `json:"username"`
	UserJID    string `json:"user_jid"`
	Symbol     string `json:"symbol"`
	Points     int    `json:"points"`
	PointsDiff int    `json:"points_diff"`
}

// Room represents an active multiplayer match instance.
type Room struct {
	ID          string     `json:"id"`
	GameType    string     `json:"game_type"` // "ttt" or "c4"
	Player1     PlayerInfo `json:"player1"`
	Player2     PlayerInfo `json:"player2"`
	TurnSession string     `json:"turn_session"`
	TurnSymbol  string     `json:"turn_symbol"`
	Status      string     `json:"status"` // "active", "finished"

	TTT *TTTGame      `json:"ttt,omitempty"`
	C4  *Connect4Game `json:"c4,omitempty"`
	WCG *WCGGame      `json:"wcg,omitempty"`

	IsBot      bool   `json:"is_bot,omitempty"`
	BotSession string `json:"bot_session,omitempty"`

	WinnerSession string    `json:"winner_session,omitempty"`
	WinnerName    string    `json:"winner_name,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	EndedAt       time.Time `json:"ended_at,omitempty"`
}

func newRoom(gameType string, p1, p2 PlayerInfo) *Room {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	roomID := "room_" + hex.EncodeToString(b)

	p1Symbol := "X"
	p2Symbol := "O"
	if gameType == "c4" {
		p1Symbol = "R"
		p2Symbol = "Y"
	} else if gameType == "wcg" {
		p1Symbol = "P1"
		p2Symbol = "P2"
	}
	p1.Symbol = p1Symbol
	p2.Symbol = p2Symbol

	r := &Room{
		ID:          roomID,
		GameType:    gameType,
		Player1:     p1,
		Player2:     p2,
		TurnSession: p1.SessionID,
		TurnSymbol:  p1Symbol,
		Status:      "active",
		CreatedAt:   time.Now(),
	}

	if gameType == "ttt" {
		r.TTT = NewTTTGame()
	} else if gameType == "c4" {
		r.C4 = NewConnect4Game()
	} else if gameType == "wcg" {
		r.WCG = NewWCGGame()
	}

	return r
}

func (r *Room) OpponentSession(sessionID string) string {
	if r.Player1.SessionID == sessionID {
		return r.Player2.SessionID
	}
	return r.Player1.SessionID
}

func (r *Room) SwitchTurn() {
	if r.TurnSession == r.Player1.SessionID {
		r.TurnSession = r.Player2.SessionID
		r.TurnSymbol = r.Player2.Symbol
	} else {
		r.TurnSession = r.Player1.SessionID
		r.TurnSymbol = r.Player1.Symbol
	}
}
