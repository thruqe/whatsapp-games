package gamelogic

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync"
	"time"

	"whatsapp-games/internal/connmanager"
	"whatsapp-games/internal/db"
)

// PlayerState tracks the in-memory activity of a player session.
type PlayerState struct {
	SessionID string
	UserJID   string
	Username  string
	Rating    int
	Status    string // "idle", "queued", "in_game"
	GameType  string
}

// Agent coordinates game state updates, matchmaking, and rules.
type Agent struct {
	connMgr *connmanager.Manager
	dbSvc   *db.Service

	playersMu sync.RWMutex
	players   map[string]*PlayerState // sessionID -> PlayerState

	queueMu     sync.Mutex
	queues      map[string][]string // gameType -> []sessionID
	queueTimers map[string]*time.Timer

	roomsMu     sync.RWMutex
	rooms       map[string]*Room   // roomID -> Room
	sessionRoom map[string]string // sessionID -> roomID
}

// NewAgent creates a new Game Logic Agent.
func NewAgent(connMgr *connmanager.Manager, dbSvc *db.Service) *Agent {
	a := &Agent{
		connMgr:     connMgr,
		dbSvc:       dbSvc,
		players:     make(map[string]*PlayerState),
		queues:      make(map[string][]string),
		queueTimers: make(map[string]*time.Timer),
		rooms:       make(map[string]*Room),
		sessionRoom: make(map[string]string),
	}
	connMgr.SetHandler(a)
	return a
}

// HandleInboundMessage processes actions dispatched by Connection Manager Agent.
func (a *Agent) HandleInboundMessage(ctx context.Context, msg connmanager.InboundMessage) {
	sessionID := msg.SessionID

	switch msg.Action {
	case "get_profile":
		a.handleGetProfile(sessionID)
	case "set_username":
		a.handleSetUsername(sessionID, msg.Payload)
	case "find_match":
		a.handleFindMatch(sessionID, msg.Payload)
	case "cancel_match":
		a.handleCancelMatch(sessionID)
	case "make_move":
		a.handleMakeMove(sessionID, msg.Payload)
	case "leave_game":
		a.handleLeaveGame(sessionID)
	default:
		_ = a.connMgr.SendToSession(sessionID, connmanager.OutboundMessage{
			Type:  "error",
			Error: fmt.Sprintf("unknown action: %s", msg.Action),
		})
	}
}

func (a *Agent) getOrCreatePlayer(sessionID string) *PlayerState {
	a.playersMu.Lock()
	defer a.playersMu.Unlock()

	p, exists := a.players[sessionID]
	if !exists {
		p = &PlayerState{
			SessionID: sessionID,
			Rating:    1000,
			Status:    "idle",
		}
		// Check if exists in DB
		if profile, err := a.dbSvc.GetProfileBySession(sessionID); err == nil && profile != nil {
			p.Username = profile.Username
			p.UserJID = profile.UserJID
			p.Rating = profile.Rating
		}
		a.players[sessionID] = p
	}
	return p
}

func (a *Agent) handleGetProfile(sessionID string) {
	p := a.getOrCreatePlayer(sessionID)
	_ = a.connMgr.SendToSession(sessionID, connmanager.OutboundMessage{
		Type: "profile",
		Payload: map[string]any{
			"session_id": p.SessionID,
			"username":   p.Username,
			"rating":     p.Rating,
			"status":     p.Status,
		},
	})
}

func (a *Agent) handleSetUsername(sessionID string, payload json.RawMessage) {
	var req struct {
		Username string `json:"username"`
		UserJID  string `json:"user_jid"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		_ = a.connMgr.SendToSession(sessionID, connmanager.OutboundMessage{
			Type:  "error",
			Error: "invalid username payload",
		})
		return
	}

	cleanName := strings.TrimSpace(req.Username)
	if len(cleanName) < 2 || len(cleanName) > 20 {
		_ = a.connMgr.SendToSession(sessionID, connmanager.OutboundMessage{
			Type:  "error",
			Error: "username must be between 2 and 20 characters",
		})
		return
	}

	ok, err := a.dbSvc.RegisterUsername(sessionID, req.UserJID, cleanName)
	if !ok || err != nil {
		errMsg := "username is already taken, please choose another"
		if err != nil && err != db.ErrUsernameTaken {
			errMsg = fmt.Sprintf("registration failed: %v", err)
		}
		_ = a.connMgr.SendToSession(sessionID, connmanager.OutboundMessage{
			Type:  "error",
			Error: errMsg,
		})
		return
	}

	a.playersMu.Lock()
	p := a.players[sessionID]
	if p == nil {
		p = &PlayerState{SessionID: sessionID, Rating: 1000, Status: "idle"}
		a.players[sessionID] = p
	}
	p.Username = cleanName
	if req.UserJID != "" {
		p.UserJID = req.UserJID
	}
	a.playersMu.Unlock()

	_ = a.connMgr.SendToSession(sessionID, connmanager.OutboundMessage{
		Type: "username_set",
		Payload: map[string]any{
			"username": cleanName,
			"rating":   p.Rating,
		},
	})
}

func (a *Agent) handleFindMatch(sessionID string, payload json.RawMessage) {
	var req struct {
		GameType string `json:"game_type"` // "ttt" or "c4"
	}
	_ = json.Unmarshal(payload, &req)
	gameType := strings.ToLower(req.GameType)
	if gameType == "" || (gameType != "ttt" && gameType != "c4") {
		gameType = "ttt"
	}

	p := a.getOrCreatePlayer(sessionID)
	if p.Username == "" {
		_ = a.connMgr.SendToSession(sessionID, connmanager.OutboundMessage{
			Type:  "error",
			Error: "please set a unique username before finding a match",
		})
		return
	}

	a.queueMu.Lock()
	defer a.queueMu.Unlock()

	// Check if already in queue
	q := a.queues[gameType]
	for _, sid := range q {
		if sid == sessionID {
			return // already waiting
		}
	}

	// If an opponent is already waiting in queue, match immediately!
	if len(q) > 0 {
		p1Session := q[0]
		a.queues[gameType] = q[1:]

		// Cancel p1's 25-second bot timer
		if timer, ok := a.queueTimers[p1Session]; ok {
			timer.Stop()
			delete(a.queueTimers, p1Session)
		}

		p.Status = "in_game"
		p.GameType = gameType

		go a.startMatch(gameType, p1Session, sessionID)
		return
	}

	// No opponent waiting yet: add to queue and start 25-second bot timer
	a.queues[gameType] = append(q, sessionID)
	p.Status = "queued"
	p.GameType = gameType

	a.queueTimers[sessionID] = time.AfterFunc(25*time.Second, func() {
		a.triggerBotMatch(sessionID, gameType)
	})

	_ = a.connMgr.SendToSession(sessionID, connmanager.OutboundMessage{
		Type: "matchmaking_status",
		Payload: map[string]any{
			"status":    "searching",
			"game_type": gameType,
			"timeout":   25,
		},
	})
}

func (a *Agent) triggerBotMatch(humanSession, gameType string) {
	a.queueMu.Lock()
	if timer, ok := a.queueTimers[humanSession]; ok {
		timer.Stop()
		delete(a.queueTimers, humanSession)
	}

	inQueue := false
	q := a.queues[gameType]
	for i, sid := range q {
		if sid == humanSession {
			a.queues[gameType] = append(q[:i], q[i+1:]...)
			inQueue = true
			break
		}
	}
	a.queueMu.Unlock()

	if !inQueue {
		return
	}

	a.playersMu.RLock()
	p, exists := a.players[humanSession]
	a.playersMu.RUnlock()
	if !exists || p.Status != "queued" {
		return
	}

	botName := GenerateUniqueBotName()
	rVal, _ := rand.Int(rand.Reader, big.NewInt(0xFFFF))
	botSession := fmt.Sprintf("bot_%04x", rVal.Int64())
	botRating := 960 + randInt(180)

	a.playersMu.Lock()
	a.players[botSession] = &PlayerState{
		SessionID: botSession,
		Username:  botName,
		Rating:    botRating,
		Status:    "in_game",
		GameType:  gameType,
	}
	a.playersMu.Unlock()

	log.Printf("[GameLogic] Matchmaking 25s expired for %s. Matching with automated bot %s (Rating: %d)",
		humanSession, botName, botRating)

	go a.startMatch(gameType, humanSession, botSession)
}

func (a *Agent) handleCancelMatch(sessionID string) {
	a.queueMu.Lock()
	if timer, ok := a.queueTimers[sessionID]; ok {
		timer.Stop()
		delete(a.queueTimers, sessionID)
	}

	for gt, q := range a.queues {
		for i, sid := range q {
			if sid == sessionID {
				a.queues[gt] = append(q[:i], q[i+1:]...)
				break
			}
		}
	}
	a.queueMu.Unlock()

	a.playersMu.Lock()
	if p, ok := a.players[sessionID]; ok && p.Status == "queued" {
		p.Status = "idle"
	}
	a.playersMu.Unlock()

	_ = a.connMgr.SendToSession(sessionID, connmanager.OutboundMessage{
		Type: "matchmaking_status",
		Payload: map[string]any{
			"status": "cancelled",
		},
	})
}

func (a *Agent) startMatch(gameType, p1Session, p2Session string) {
	p1State := a.getOrCreatePlayer(p1Session)
	p2State := a.getOrCreatePlayer(p2Session)

	p1Info := PlayerInfo{
		SessionID: p1Session,
		Username:  p1State.Username,
		UserJID:   p1State.UserJID,
		Rating:    p1State.Rating,
	}
	p2Info := PlayerInfo{
		SessionID: p2Session,
		Username:  p2State.Username,
		UserJID:   p2State.UserJID,
		Rating:    p2State.Rating,
	}

	room := newRoom(gameType, p1Info, p2Info)
	if strings.HasPrefix(p2Session, "bot_") {
		room.IsBot = true
		room.BotSession = p2Session
	}

	a.roomsMu.Lock()
	a.rooms[room.ID] = room
	a.sessionRoom[p1Session] = room.ID
	a.sessionRoom[p2Session] = room.ID
	a.roomsMu.Unlock()

	p1State.Status = "in_game"
	p2State.Status = "in_game"

	log.Printf("[GameLogic] Match started: room=%s game=%s %s vs %s (isBot: %v)",
		room.ID, gameType, p1Info.Username, p2Info.Username, room.IsBot)

	// Notify player 1
	_ = a.connMgr.SendToSession(p1Session, connmanager.OutboundMessage{
		Type: "match_start",
		Payload: map[string]any{
			"room_id":     room.ID,
			"game_type":   gameType,
			"you":         room.Player1,
			"opponent":    room.Player2,
			"your_turn":   room.TurnSession == p1Session,
			"turn_symbol": room.TurnSymbol,
			"board":       a.getGameBoard(room),
		},
	})

	if !room.IsBot {
		// Notify player 2
		_ = a.connMgr.SendToSession(p2Session, connmanager.OutboundMessage{
			Type: "match_start",
			Payload: map[string]any{
				"room_id":     room.ID,
				"game_type":   gameType,
				"you":         room.Player2,
				"opponent":    room.Player1,
				"your_turn":   room.TurnSession == p2Session,
				"turn_symbol": room.TurnSymbol,
				"board":       a.getGameBoard(room),
			},
		})
	} else if room.TurnSession == room.BotSession {
		go a.scheduleBotMove(room.ID)
	}
}

func (a *Agent) getGameBoard(room *Room) any {
	if room.GameType == "ttt" && room.TTT != nil {
		return room.TTT.Board
	} else if room.GameType == "c4" && room.C4 != nil {
		return room.C4.Board
	}
	return nil
}

func (a *Agent) handleMakeMove(sessionID string, payload json.RawMessage) {
	a.roomsMu.RLock()
	roomID, inRoom := a.sessionRoom[sessionID]
	var room *Room
	if inRoom {
		room = a.rooms[roomID]
	}
	a.roomsMu.RUnlock()

	if !inRoom || room == nil || room.Status != "active" {
		_ = a.connMgr.SendToSession(sessionID, connmanager.OutboundMessage{
			Type:  "error",
			Error: "no active match",
		})
		return
	}

	if room.TurnSession != sessionID {
		_ = a.connMgr.SendToSession(sessionID, connmanager.OutboundMessage{
			Type:  "error",
			Error: "it is not your turn",
		})
		return
	}

	var moveResult string
	var err error

	if room.GameType == "ttt" {
		var req struct {
			Pos int `json:"pos"` // 0-8
		}
		if err = json.Unmarshal(payload, &req); err != nil {
			_ = a.connMgr.SendToSession(sessionID, connmanager.OutboundMessage{Type: "error", Error: "invalid move"})
			return
		}
		err = room.TTT.MakeMove(req.Pos, room.TurnSymbol)
		if err == nil {
			moveResult = room.TTT.CheckWinner()
		}
	} else if room.GameType == "c4" {
		var req struct {
			Col int `json:"col"` // 0-6
		}
		if err = json.Unmarshal(payload, &req); err != nil {
			_ = a.connMgr.SendToSession(sessionID, connmanager.OutboundMessage{Type: "error", Error: "invalid move"})
			return
		}
		_, err = room.C4.DropPiece(req.Col, room.TurnSymbol)
		if err == nil {
			moveResult = room.C4.CheckWinner()
		}
	}

	if err != nil {
		_ = a.connMgr.SendToSession(sessionID, connmanager.OutboundMessage{
			Type:  "error",
			Error: err.Error(),
		})
		return
	}

	// Check if game ended
	if moveResult != "" {
		a.finishMatch(room, moveResult)
		return
	}

	// Game continues: switch turn and broadcast state update
	room.SwitchTurn()
	board := a.getGameBoard(room)

	_ = a.connMgr.SendToSession(room.Player1.SessionID, connmanager.OutboundMessage{
		Type: "move_update",
		Payload: map[string]any{
			"board":       board,
			"your_turn":   room.TurnSession == room.Player1.SessionID,
			"turn_symbol": room.TurnSymbol,
		},
	})

	if !room.IsBot {
		_ = a.connMgr.SendToSession(room.Player2.SessionID, connmanager.OutboundMessage{
			Type: "move_update",
			Payload: map[string]any{
				"board":       board,
				"your_turn":   room.TurnSession == room.Player2.SessionID,
				"turn_symbol": room.TurnSymbol,
			},
		})
	} else if room.TurnSession == room.BotSession {
		go a.scheduleBotMove(room.ID)
	}
}

func (a *Agent) finishMatch(room *Room, winnerSymbol string) {
	room.Status = "finished"
	room.EndedAt = time.Now()

	p1Session := room.Player1.SessionID
	p2Session := room.Player2.SessionID

	p1Delta := 0
	p2Delta := 0
	winnerSession := ""
	winnerName := ""

	if winnerSymbol == "draw" {
		p1Delta = 5
		p2Delta = 5
		winnerName = "Draw"
	} else if winnerSymbol == room.Player1.Symbol {
		winnerSession = p1Session
		winnerName = room.Player1.Username
		p1Delta = 25
		p2Delta = -15
	} else {
		winnerSession = p2Session
		winnerName = room.Player2.Username
		p2Delta = 25
		p1Delta = -15
	}

	room.WinnerSession = winnerSession
	room.WinnerName = winnerName

	moves := 0
	if room.TTT != nil {
		moves = room.TTT.MoveCount
	} else if room.C4 != nil {
		moves = room.C4.MoveCount
	}

	// Dispatch InteractionEvent to Database Service through buffered channel
	evt := db.InteractionEvent{
		Type:           db.EventMatchResult,
		Timestamp:      time.Now(),
		GameType:       room.GameType,
		Player1Session: p1Session,
		Player2Session: p2Session,
		Player1Name:    room.Player1.Username,
		Player2Name:    room.Player2.Username,
		WinnerSession:  winnerSession,
		MovesCount:     moves,
		Player1Delta:   p1Delta,
		Player2Delta:   p2Delta,
	}
	a.dbSvc.DispatchEvent(evt)

	// Update local player state ratings
	a.playersMu.Lock()
	if p1 := a.players[p1Session]; p1 != nil {
		p1.Rating = max(100, p1.Rating+p1Delta)
		p1.Status = "idle"
	}
	if p2 := a.players[p2Session]; p2 != nil {
		p2.Rating = max(100, p2.Rating+p2Delta)
		p2.Status = "idle"
	}
	a.playersMu.Unlock()

	// Clear session room mappings
	a.roomsMu.Lock()
	delete(a.sessionRoom, p1Session)
	delete(a.sessionRoom, p2Session)
	a.roomsMu.Unlock()

	board := a.getGameBoard(room)

	// Broadcast game_over to player 1
	_ = a.connMgr.SendToSession(p1Session, connmanager.OutboundMessage{
		Type: "game_over",
		Payload: map[string]any{
			"board":          board,
			"winner_session": winnerSession,
			"winner_name":    winnerName,
			"is_winner":      winnerSession == p1Session,
			"is_draw":        winnerSymbol == "draw",
			"rating_delta":   p1Delta,
		},
	})

	// Broadcast game_over to player 2
	_ = a.connMgr.SendToSession(p2Session, connmanager.OutboundMessage{
		Type: "game_over",
		Payload: map[string]any{
			"board":          board,
			"winner_session": winnerSession,
			"winner_name":    winnerName,
			"is_winner":      winnerSession == p2Session,
			"is_draw":        winnerSymbol == "draw",
			"rating_delta":   p2Delta,
		},
	})
}

func (a *Agent) handleLeaveGame(sessionID string) {
	a.handleClientLeave(sessionID, "resigned")
}

// HandleClientDisconnect handles a client WebSocket dropping.
func (a *Agent) HandleClientDisconnect(sessionID string) {
	a.handleCancelMatch(sessionID)
	a.handleClientLeave(sessionID, "disconnected")
}

func (a *Agent) handleClientLeave(sessionID, reason string) {
	a.roomsMu.RLock()
	roomID, inRoom := a.sessionRoom[sessionID]
	var room *Room
	if inRoom {
		room = a.rooms[roomID]
	}
	a.roomsMu.RUnlock()

	if !inRoom || room == nil || room.Status != "active" {
		return
	}

	opponentSession := room.OpponentSession(sessionID)
	log.Printf("[GameLogic] Player %s %s from room %s. Forfeit win to %s",
		sessionID, reason, room.ID, opponentSession)

	// Opponent wins by forfeit
	winnerSymbol := room.Player1.Symbol
	if opponentSession == room.Player2.SessionID {
		winnerSymbol = room.Player2.Symbol
	}
	a.finishMatch(room, winnerSymbol)
}

func (a *Agent) scheduleBotMove(roomID string) {
	// Realistic reaction delay between 700ms and 1400ms
	delay := 700 + randInt(700)
	time.Sleep(time.Duration(delay) * time.Millisecond)

	a.roomsMu.RLock()
	room, exists := a.rooms[roomID]
	a.roomsMu.RUnlock()

	if !exists || room == nil || room.Status != "active" || !room.IsBot {
		return
	}
	if room.TurnSession != room.BotSession {
		return
	}

	botSymbol := room.Player2.Symbol
	humanSymbol := room.Player1.Symbol

	var moveResult string
	var err error

	if room.GameType == "ttt" && room.TTT != nil {
		pos := SelectTTTMove(room.TTT.Board, botSymbol, humanSymbol)
		if pos >= 0 {
			err = room.TTT.MakeMove(pos, botSymbol)
			if err == nil {
				moveResult = room.TTT.CheckWinner()
			}
		}
	} else if room.GameType == "c4" && room.C4 != nil {
		col := SelectC4Move(room.C4.Board, botSymbol, humanSymbol)
		if col >= 0 {
			_, err = room.C4.DropPiece(col, botSymbol)
			if err == nil {
				moveResult = room.C4.CheckWinner()
			}
		}
	}

	if err != nil {
		log.Printf("[GameLogic] Bot move error in room %s: %v", room.ID, err)
		return
	}

	if moveResult != "" {
		a.finishMatch(room, moveResult)
		return
	}

	// Switch turn back to human player
	room.SwitchTurn()
	board := a.getGameBoard(room)

	_ = a.connMgr.SendToSession(room.Player1.SessionID, connmanager.OutboundMessage{
		Type: "move_update",
		Payload: map[string]any{
			"board":       board,
			"your_turn":   room.TurnSession == room.Player1.SessionID,
			"turn_symbol": room.TurnSymbol,
		},
	})
}

func randInt(n int) int {
	if n <= 0 {
		return 0
	}
	b, _ := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if b == nil {
		return int(time.Now().UnixNano() % int64(n))
	}
	return int(b.Int64())
}

