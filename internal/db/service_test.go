package db

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDatabaseService(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_games.db")

	svc, err := NewService(dbPath, 100)
	if err != nil {
		t.Fatalf("failed to init db service: %v", err)
	}
	defer svc.Close()

	// Test 1: Register username
	ok, err := svc.RegisterUsername("sess_1", "12345@s.whatsapp.net", "Player1")
	if !ok || err != nil {
		t.Fatalf("expected successful registration, got ok=%v err=%v", ok, err)
	}

	// Test 2: Duplicate username rejection
	ok2, err2 := svc.RegisterUsername("sess_2", "67890@s.whatsapp.net", "Player1")
	if ok2 || err2 != ErrUsernameTaken {
		t.Fatalf("expected ErrUsernameTaken for duplicate username, got ok=%v err=%v", ok2, err2)
	}

	// Test 3: Second unique user registration
	ok3, err3 := svc.RegisterUsername("sess_2", "67890@s.whatsapp.net", "Player2")
	if !ok3 || err3 != nil {
		t.Fatalf("expected successful registration for player2: %v", err3)
	}

	// Test 4: Profile fetch
	p1, err := svc.GetProfileBySession("sess_1")
	if err != nil || p1.Username != "Player1" || p1.Rating != 1000 {
		t.Fatalf("unexpected profile: %+v, err=%v", p1, err)
	}

	// Test 5: Interaction event dispatch through buffered channel
	matchEvt := InteractionEvent{
		Type:           EventMatchResult,
		Timestamp:      time.Now(),
		GameType:       "ttt",
		Player1Session: "sess_1",
		Player2Session: "sess_2",
		Player1Name:    "Player1",
		Player2Name:    "Player2",
		WinnerSession:  "sess_1",
		MovesCount:     5,
		Player1Delta:   25,
		Player2Delta:   -15,
	}

	dispatched := svc.DispatchEvent(matchEvt)
	if !dispatched {
		t.Fatalf("expected event to be dispatched into buffered channel")
	}

	// Wait for batch flush interval (500ms)
	time.Sleep(700 * time.Millisecond)

	// Verify ratings updated in DB
	p1Updated, err := svc.GetProfileBySession("sess_1")
	if err != nil || p1Updated.Wins != 1 || p1Updated.Rating != 1025 {
		t.Fatalf("expected Player1 to have 1 win and 1025 rating, got %+v", p1Updated)
	}

	p2Updated, err := svc.GetProfileBySession("sess_2")
	if err != nil || p2Updated.Losses != 1 || p2Updated.Rating != 985 {
		t.Fatalf("expected Player2 to have 1 loss and 985 rating, got %+v", p2Updated)
	}

	// Test 6: Leaderboard
	lb, err := svc.GetLeaderboard(10)
	if err != nil || len(lb) != 2 {
		t.Fatalf("expected 2 leaderboard entries, got %d, err=%v", len(lb), err)
	}
	if lb[0].Username != "Player1" || lb[0].Rating != 1025 {
		t.Fatalf("expected Player1 to lead leaderboard, got %+v", lb[0])
	}
}
