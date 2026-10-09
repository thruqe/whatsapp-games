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

	// Test 4: Profile fetch (initial points should be 0, diff 0)
	p1, err := svc.GetProfileBySession("sess_1")
	if err != nil || p1.Username != "Player1" || p1.Points != 0 || p1.PointsDiff != 0 {
		t.Fatalf("unexpected profile: %+v, err=%v", p1, err)
	}

	// Test 5: Interaction event dispatch through buffered channel (Points: +25 for winner, 0 for loser, PointsDiff: +25 / -25)
	matchEvt := InteractionEvent{
		Type:              EventMatchResult,
		Timestamp:         time.Now(),
		GameType:          "ttt",
		Player1Session:    "sess_1",
		Player2Session:    "sess_2",
		Player1Name:       "Player1",
		Player2Name:       "Player2",
		WinnerSession:     "sess_1",
		MovesCount:        5,
		Player1Points:     25,
		Player2Points:     0,
		Player1PointsDiff: 25,
		Player2PointsDiff: -25,
	}

	dispatched := svc.DispatchEvent(matchEvt)
	if !dispatched {
		t.Fatalf("expected event to be dispatched into buffered channel")
	}

	// Wait for batch flush interval (500ms)
	time.Sleep(700 * time.Millisecond)

	// Verify points and points diff updated in DB
	p1Updated, err := svc.GetProfileBySession("sess_1")
	if err != nil || p1Updated.Wins != 1 || p1Updated.Points != 25 || p1Updated.PointsDiff != 25 {
		t.Fatalf("expected Player1 to have 1 win, 25 points, and +25 points diff, got %+v", p1Updated)
	}

	p2Updated, err := svc.GetProfileBySession("sess_2")
	if err != nil || p2Updated.Losses != 1 || p2Updated.Points != 0 || p2Updated.PointsDiff != -25 {
		t.Fatalf("expected Player2 to have 1 loss, 0 points, and -25 points diff, got %+v", p2Updated)
	}

	// Test 6: Global Leaderboard
	globalLb, err := svc.GetLeaderboard("global", 10)
	if err != nil || len(globalLb) != 2 {
		t.Fatalf("expected 2 global leaderboard entries, got %d, err=%v", len(globalLb), err)
	}
	if globalLb[0].Username != "Player1" || globalLb[0].Points != 25 || globalLb[0].PointsDiff != 25 {
		t.Fatalf("expected Player1 to lead global leaderboard, got %+v", globalLb[0])
	}

	// Test 7: Game-Specific Leaderboard (ttt)
	tttLb, err := svc.GetLeaderboard("ttt", 10)
	if err != nil || len(tttLb) != 2 {
		t.Fatalf("expected 2 ttt leaderboard entries, got %d, err=%v", len(tttLb), err)
	}
	if tttLb[0].Username != "Player1" || tttLb[0].Points != 25 || tttLb[0].PointsDiff != 25 {
		t.Fatalf("expected Player1 to lead ttt leaderboard, got %+v", tttLb[0])
	}

	// Test 8: Empty game leaderboard for c4
	c4Lb, err := svc.GetLeaderboard("c4", 10)
	if err != nil || len(c4Lb) != 0 {
		t.Fatalf("expected 0 c4 leaderboard entries, got %d", len(c4Lb))
	}
}
