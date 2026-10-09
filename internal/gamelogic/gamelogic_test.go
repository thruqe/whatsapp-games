package gamelogic

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"whatsapp-games/internal/connmanager"
	"whatsapp-games/internal/db"
)

func TestTTTEngine(t *testing.T) {
	game := NewTTTGame()

	// Move 1: X at 0
	if err := game.MakeMove(0, "X"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Move 2: O at 3
	if err := game.MakeMove(3, "O"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Move 3: X at 1
	if err := game.MakeMove(1, "X"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Move 4: O at 4
	if err := game.MakeMove(4, "O"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Move 5: X at 2 (wins row 0)
	if err := game.MakeMove(2, "X"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	winner := game.CheckWinner()
	if winner != "X" {
		t.Fatalf("expected X to win, got: %s", winner)
	}
}

func TestConnect4Engine(t *testing.T) {
	c4 := NewConnect4Game()

	// Drop 4 Red pieces in column 0 vertically
	for i := 0; i < 4; i++ {
		row, err := c4.DropPiece(0, "R")
		if err != nil {
			t.Fatalf("failed to drop piece %d: %v", i, err)
		}
		expectedRow := 5 - i
		if row != expectedRow {
			t.Fatalf("expected piece to land in row %d, got %d", expectedRow, row)
		}
	}

	winner := c4.CheckWinner()
	if winner != "R" {
		t.Fatalf("expected R to win vertically, got: %s", winner)
	}
}

func TestGameLogicAgentMatchFlow(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	dbSvc, err := db.NewService(dbPath, 100)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer dbSvc.Close()

	connMgr := connmanager.NewManager(nil)
	agent := NewAgent(connMgr, dbSvc)

	ctx := context.Background()

	// 1. Set username for Player 1
	agent.HandleInboundMessage(ctx, connmanager.InboundMessage{
		SessionID: "sess_p1",
		Action:    "set_username",
		Payload:   json.RawMessage(`{"username":"Alice"}`),
	})

	// 2. Set username for Player 2
	agent.HandleInboundMessage(ctx, connmanager.InboundMessage{
		SessionID: "sess_p2",
		Action:    "set_username",
		Payload:   json.RawMessage(`{"username":"Bob"}`),
	})

	// 3. Queue both for Tic-Tac-Toe
	agent.HandleInboundMessage(ctx, connmanager.InboundMessage{
		SessionID: "sess_p1",
		Action:    "find_match",
		Payload:   json.RawMessage(`{"game_type":"ttt"}`),
	})
	agent.HandleInboundMessage(ctx, connmanager.InboundMessage{
		SessionID: "sess_p2",
		Action:    "find_match",
		Payload:   json.RawMessage(`{"game_type":"ttt"}`),
	})

	// Give a small moment for match creation
	time.Sleep(50 * time.Millisecond)

	agent.roomsMu.RLock()
	roomID, ok1 := agent.sessionRoom["sess_p1"]
	room := agent.rooms[roomID]
	agent.roomsMu.RUnlock()

	if !ok1 || room == nil || room.Status != "active" {
		t.Fatalf("expected active match for p1 and p2")
	}

	// 4. Play Tic-Tac-Toe moves until Alice (P1) wins
	// P1: pos 0
	agent.HandleInboundMessage(ctx, connmanager.InboundMessage{
		SessionID: "sess_p1",
		Action:    "make_move",
		Payload:   json.RawMessage(`{"pos":0}`),
	})
	// P2: pos 3
	agent.HandleInboundMessage(ctx, connmanager.InboundMessage{
		SessionID: "sess_p2",
		Action:    "make_move",
		Payload:   json.RawMessage(`{"pos":3}`),
	})
	// P1: pos 1
	agent.HandleInboundMessage(ctx, connmanager.InboundMessage{
		SessionID: "sess_p1",
		Action:    "make_move",
		Payload:   json.RawMessage(`{"pos":1}`),
	})
	// P2: pos 4
	agent.HandleInboundMessage(ctx, connmanager.InboundMessage{
		SessionID: "sess_p2",
		Action:    "make_move",
		Payload:   json.RawMessage(`{"pos":4}`),
	})
	// P1: pos 2 -> Win!
	agent.HandleInboundMessage(ctx, connmanager.InboundMessage{
		SessionID: "sess_p1",
		Action:    "make_move",
		Payload:   json.RawMessage(`{"pos":2}`),
	})

	if room.Status != "finished" || room.WinnerSession != "sess_p1" {
		t.Fatalf("expected room finished with winner p1, got: status=%s winner=%s", room.Status, room.WinnerSession)
	}

	// 5. Check if interaction event reached database through buffered channel
	time.Sleep(600 * time.Millisecond) // wait for DB batch worker

	p1Prof, err := dbSvc.GetProfileBySession("sess_p1")
	if err != nil || p1Prof.Wins != 1 || p1Prof.Rating != 1025 {
		t.Fatalf("expected p1 to have 1 win and 1025 rating, got %+v, err=%v", p1Prof, err)
	}

	p2Prof, err := dbSvc.GetProfileBySession("sess_p2")
	if err != nil || p2Prof.Losses != 1 || p2Prof.Rating != 985 {
		t.Fatalf("expected p2 to have 1 loss and 985 rating, got %+v, err=%v", p2Prof, err)
	}
}

func TestBotEngineAndNames(t *testing.T) {
	// Test bot names are unique and generated with esoteric patterns
	names := make(map[string]bool)
	for i := 0; i < 20; i++ {
		name := GenerateUniqueBotName()
		if name == "" {
			t.Fatalf("expected non-empty bot name")
		}
		if names[name] {
			t.Fatalf("duplicate bot name generated: %s", name)
		}
		names[name] = true
	}

	// Test TTT Bot winning move detection
	// Board: O at 0, 1 -> Bot O must play 2 to win!
	var tttBoard [9]string
	tttBoard[0] = "O"
	tttBoard[1] = "O"
	move := SelectTTTMove(tttBoard, "O", "X")
	if move != 2 {
		t.Fatalf("expected bot to choose winning move 2, got %d", move)
	}

	// Test TTT Bot blocking move detection
	// Board: X at 3, 4 -> Bot O must block at 5!
	var tttBlock [9]string
	tttBlock[3] = "X"
	tttBlock[4] = "X"
	blockMove := SelectTTTMove(tttBlock, "O", "X")
	if blockMove != 5 {
		t.Fatalf("expected bot to block at 5, got %d", blockMove)
	}

	// Test C4 Bot winning drop detection
	// Board: 3 Red pieces in col 3 -> Bot R must drop into col 3 to win!
	var c4Board [C4Rows][C4Cols]string
	c4Board[5][3] = "R"
	c4Board[4][3] = "R"
	c4Board[3][3] = "R"
	c4Move := SelectC4Move(c4Board, "R", "Y")
	if c4Move != 3 {
		t.Fatalf("expected bot to choose winning column 3, got %d", c4Move)
	}

	// Test C4 Bot blocking drop detection
	// Board: 3 Yellow pieces in col 1 -> Bot R must drop into col 1 to block!
	var c4Block [C4Rows][C4Cols]string
	c4Block[5][1] = "Y"
	c4Block[4][1] = "Y"
	c4Block[3][1] = "Y"
	c4BlockMove := SelectC4Move(c4Block, "R", "Y")
	if c4BlockMove != 1 {
		t.Fatalf("expected bot to block in column 1, got %d", c4BlockMove)
	}
}

func TestBotMatchmaking(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "bot_test.db")

	dbSvc, err := db.NewService(dbPath, 100)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer dbSvc.Close()

	connMgr := connmanager.NewManager(nil)
	agent := NewAgent(connMgr, dbSvc)

	ctx := context.Background()
	_, _ = dbSvc.RegisterUsername("sess_human", "human@s.whatsapp.net", "HumanPlayer")

	// Trigger bot match directly
	agent.playersMu.Lock()
	agent.players["sess_human"] = &PlayerState{
		SessionID: "sess_human",
		Username:  "HumanPlayer",
		Rating:    1000,
		Status:    "queued",
		GameType:  "ttt",
	}
	agent.queues["ttt"] = []string{"sess_human"}
	agent.playersMu.Unlock()

	agent.triggerBotMatch("sess_human", "ttt")

	time.Sleep(50 * time.Millisecond)

	agent.roomsMu.RLock()
	roomID, inRoom := agent.sessionRoom["sess_human"]
	room := agent.rooms[roomID]
	agent.roomsMu.RUnlock()

	if !inRoom || room == nil {
		t.Fatalf("expected human to be matched into a room")
	}

	if !room.IsBot {
		t.Fatalf("expected room.IsBot to be true")
	}

	if room.Player2.Username == "" || room.Player2.Username == "HumanPlayer" {
		t.Fatalf("expected unique bot opponent, got %s", room.Player2.Username)
	}

	// Test human makes move 0, bot should respond with its move
	agent.HandleInboundMessage(ctx, connmanager.InboundMessage{
		SessionID: "sess_human",
		Action:    "make_move",
		Payload:   json.RawMessage(`{"pos":0}`),
	})

	// Wait for bot's reaction delay + execution
	time.Sleep(1600 * time.Millisecond)

	if room.TTT.MoveCount < 2 {
		t.Fatalf("expected bot to have executed its move, moveCount=%d", room.TTT.MoveCount)
	}
}

