package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

var (
	ErrUsernameTaken = errors.New("username is already taken")
	ErrNotFound      = errors.New("record not found")
)

// Service manages database persistence and asynchronous event batching.
type Service struct {
	db        *sql.DB
	eventChan chan InteractionEvent
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup

	// Cache for fast username uniqueness checks before batching
	namesMu sync.RWMutex
	nameMap map[string]string // lowercase username -> sessionID
}

// NewService initializes SQLite with WAL mode and launches the asynchronous batch worker.
func NewService(dbPath string, bufferSize int) (*Service, error) {
	if bufferSize <= 0 {
		bufferSize = 1024
	}

	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	db.SetMaxOpenConns(1) // SQLite performs best with 1 writer connection

	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{
		db:        db,
		eventChan: make(chan InteractionEvent, bufferSize),
		ctx:       ctx,
		cancel:    cancel,
		nameMap:   make(map[string]string),
	}

	if err := s.initSchema(); err != nil {
		_ = db.Close()
		cancel()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	if err := s.preloadUsernames(); err != nil {
		log.Printf("[DB] Warning: failed to preload usernames: %v", err)
	}

	s.wg.Add(1)
	go s.batchWorker()

	return s, nil
}

func (s *Service) initSchema() error {
	createTables := `
	CREATE TABLE IF NOT EXISTS users (
		session_id TEXT PRIMARY KEY,
		user_jid TEXT NOT NULL,
		username TEXT UNIQUE NOT NULL,
		points INTEGER NOT NULL DEFAULT 0,
		wins INTEGER NOT NULL DEFAULT 0,
		losses INTEGER NOT NULL DEFAULT 0,
		draws INTEGER NOT NULL DEFAULT 0,
		points_diff INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS game_stats (
		session_id TEXT NOT NULL,
		game_type TEXT NOT NULL,
		points INTEGER NOT NULL DEFAULT 0,
		wins INTEGER NOT NULL DEFAULT 0,
		losses INTEGER NOT NULL DEFAULT 0,
		draws INTEGER NOT NULL DEFAULT 0,
		points_diff INTEGER NOT NULL DEFAULT 0,
		updated_at DATETIME NOT NULL,
		PRIMARY KEY (session_id, game_type)
	);

	CREATE TABLE IF NOT EXISTS matches (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		game_type TEXT NOT NULL,
		player1_session TEXT NOT NULL,
		player2_session TEXT NOT NULL,
		player1_name TEXT NOT NULL,
		player2_name TEXT NOT NULL,
		winner_session TEXT NOT NULL,
		moves_count INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL
	);
	`
	if _, err := s.db.Exec(createTables); err != nil {
		return err
	}

	// Safe migration for pre-existing tables lacking points or points_diff
	_, _ = s.db.Exec(`ALTER TABLE users ADD COLUMN points INTEGER NOT NULL DEFAULT 0;`)
	_, _ = s.db.Exec(`ALTER TABLE users ADD COLUMN points_diff INTEGER NOT NULL DEFAULT 0;`)

	createIndexes := `
	CREATE INDEX IF NOT EXISTS idx_users_points ON users (points DESC, points_diff DESC);
	CREATE INDEX IF NOT EXISTS idx_users_username ON users (username);
	CREATE INDEX IF NOT EXISTS idx_game_stats_points ON game_stats (game_type, points DESC, points_diff DESC);
	`
	if _, err := s.db.Exec(createIndexes); err != nil {
		return err
	}

	return nil
}

func (s *Service) preloadUsernames() error {
	rows, err := s.db.Query("SELECT session_id, username FROM users")
	if err != nil {
		return err
	}
	defer rows.Close()

	s.namesMu.Lock()
	defer s.namesMu.Unlock()
	for rows.Next() {
		var sess, name string
		if err := rows.Scan(&sess, &name); err == nil {
			s.nameMap[strings.ToLower(name)] = sess
		}
	}
	return nil
}

// EventChannel returns the buffered channel for Game Logic Agent to send interaction events into.
func (s *Service) EventChannel() chan<- InteractionEvent {
	return s.eventChan
}

// DispatchEvent sends an event into the buffered channel non-blockingly.
func (s *Service) DispatchEvent(evt InteractionEvent) bool {
	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now()
	}
	select {
	case s.eventChan <- evt:
		return true
	default:
		log.Printf("[DB] Warning: Event channel full, dropping event: %v", evt.Type)
		return false
	}
}

// RegisterUsername attempts to claim a unique username for a session ID.
func (s *Service) RegisterUsername(sessionID, userJID, username string) (bool, error) {
	cleanName := strings.TrimSpace(username)
	if len(cleanName) < 2 || len(cleanName) > 20 {
		return false, fmt.Errorf("username must be between 2 and 20 characters")
	}

	lower := strings.ToLower(cleanName)

	s.namesMu.Lock()
	existingSession, exists := s.nameMap[lower]
	if exists && existingSession != sessionID {
		s.namesMu.Unlock()
		return false, ErrUsernameTaken
	}
	s.nameMap[lower] = sessionID
	s.namesMu.Unlock()

	// Direct DB upsert to guarantee persistence immediately for authentication
	now := time.Now()
	query := `
	INSERT INTO users (session_id, user_jid, username, points, wins, losses, draws, points_diff, created_at, updated_at)
	VALUES (?, ?, ?, 0, 0, 0, 0, 0, ?, ?)
	ON CONFLICT(session_id) DO UPDATE SET
		username = excluded.username,
		updated_at = excluded.updated_at;
	`
	_, err := s.db.Exec(query, sessionID, userJID, cleanName, now, now)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			s.namesMu.Lock()
			delete(s.nameMap, lower)
			s.namesMu.Unlock()
			return false, ErrUsernameTaken
		}
		return false, fmt.Errorf("register user in db: %w", err)
	}

	return true, nil
}

// GetProfileBySession fetches a user profile by their bot-provided session ID.
func (s *Service) GetProfileBySession(sessionID string) (*UserProfile, error) {
	row := s.db.QueryRow(`
		SELECT session_id, user_jid, username, points, wins, losses, draws, points_diff, created_at, updated_at
		FROM users WHERE session_id = ?`, sessionID)

	var p UserProfile
	err := row.Scan(&p.SessionID, &p.UserJID, &p.Username, &p.Points, &p.Wins, &p.Losses, &p.Draws, &p.PointsDiff, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetLeaderboard returns the top ranked players globally or for a specific game.
func (s *Service) GetLeaderboard(gameType string, limit int) ([]LeaderboardEntry, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	gameType = strings.ToLower(strings.TrimSpace(gameType))

	var rows *sql.Rows
	var err error

	if gameType == "" || gameType == "global" || gameType == "all" {
		rows, err = s.db.Query(`
			SELECT username, 'global' as game_type, points, wins, losses, draws, points_diff
			FROM users
			ORDER BY points DESC, points_diff DESC, wins DESC
			LIMIT ?`, limit)
	} else {
		rows, err = s.db.Query(`
			SELECT u.username, s.game_type, s.points, s.wins, s.losses, s.draws, s.points_diff
			FROM game_stats s
			JOIN users u ON s.session_id = u.session_id
			WHERE s.game_type = ?
			ORDER BY s.points DESC, s.points_diff DESC, s.wins DESC
			LIMIT ?`, gameType, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]LeaderboardEntry, 0)
	for rows.Next() {
		var e LeaderboardEntry
		if err := rows.Scan(&e.Username, &e.GameType, &e.Points, &e.Wins, &e.Losses, &e.Draws, &e.PointsDiff); err != nil {
			return nil, err
		}
		list = append(list, e)
	}
	return list, nil
}

// batchWorker flushes interaction events periodically in database transactions.
func (s *Service) batchWorker() {
	defer s.wg.Done()

	const (
		batchSize     = 25
		flushInterval = 500 * time.Millisecond
	)

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	var batch []InteractionEvent

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := s.persistBatch(batch); err != nil {
			log.Printf("[DB] Error persisting event batch of %d items: %v", len(batch), err)
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-s.ctx.Done():
			// Drain remaining events in channel before terminating
			for {
				select {
				case evt := <-s.eventChan:
					batch = append(batch, evt)
				default:
					flush()
					return
				}
			}

		case evt := <-s.eventChan:
			batch = append(batch, evt)
			if len(batch) >= batchSize {
				flush()
			}

		case <-ticker.C:
			flush()
		}
	}
}

// persistBatch writes a batch of interaction events inside a single transaction.
func (s *Service) persistBatch(events []InteractionEvent) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	matchStmt, err := tx.Prepare(`
		INSERT INTO matches (game_type, player1_session, player2_session, player1_name, player2_name, winner_session, moves_count, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare match stmt: %w", err)
	}
	defer matchStmt.Close()

	updateUserWinStmt, err := tx.Prepare(`
		UPDATE users SET
			points = points + ?,
			points_diff = points_diff + ?,
			wins = wins + 1,
			updated_at = ?
		WHERE session_id = ?
	`)
	if err != nil {
		return fmt.Errorf("prepare user win stmt: %w", err)
	}
	defer updateUserWinStmt.Close()

	updateUserLossStmt, err := tx.Prepare(`
		UPDATE users SET
			points = points + ?,
			points_diff = points_diff + ?,
			losses = losses + 1,
			updated_at = ?
		WHERE session_id = ?
	`)
	if err != nil {
		return fmt.Errorf("prepare user loss stmt: %w", err)
	}
	defer updateUserLossStmt.Close()

	updateUserDrawStmt, err := tx.Prepare(`
		UPDATE users SET
			points = points + ?,
			points_diff = points_diff + ?,
			draws = draws + 1,
			updated_at = ?
		WHERE session_id = ?
	`)
	if err != nil {
		return fmt.Errorf("prepare user draw stmt: %w", err)
	}
	defer updateUserDrawStmt.Close()

	upsertGameStatsWinStmt, err := tx.Prepare(`
		INSERT INTO game_stats (session_id, game_type, points, wins, losses, draws, points_diff, updated_at)
		VALUES (?, ?, ?, 1, 0, 0, ?, ?)
		ON CONFLICT(session_id, game_type) DO UPDATE SET
			points = points + excluded.points,
			points_diff = points_diff + excluded.points_diff,
			wins = wins + 1,
			updated_at = excluded.updated_at
	`)
	if err != nil {
		return fmt.Errorf("prepare game_stats win stmt: %w", err)
	}
	defer upsertGameStatsWinStmt.Close()

	upsertGameStatsLossStmt, err := tx.Prepare(`
		INSERT INTO game_stats (session_id, game_type, points, wins, losses, draws, points_diff, updated_at)
		VALUES (?, ?, ?, 0, 1, 0, ?, ?)
		ON CONFLICT(session_id, game_type) DO UPDATE SET
			points = points + excluded.points,
			points_diff = points_diff + excluded.points_diff,
			losses = losses + 1,
			updated_at = excluded.updated_at
	`)
	if err != nil {
		return fmt.Errorf("prepare game_stats loss stmt: %w", err)
	}
	defer upsertGameStatsLossStmt.Close()

	upsertGameStatsDrawStmt, err := tx.Prepare(`
		INSERT INTO game_stats (session_id, game_type, points, wins, losses, draws, points_diff, updated_at)
		VALUES (?, ?, ?, 0, 0, 1, ?, ?)
		ON CONFLICT(session_id, game_type) DO UPDATE SET
			points = points + excluded.points,
			points_diff = points_diff + excluded.points_diff,
			draws = draws + 1,
			updated_at = excluded.updated_at
	`)
	if err != nil {
		return fmt.Errorf("prepare game_stats draw stmt: %w", err)
	}
	defer upsertGameStatsDrawStmt.Close()

	now := time.Now()
	for _, evt := range events {
		switch evt.Type {
		case EventMatchResult:
			_, err := matchStmt.Exec(
				evt.GameType,
				evt.Player1Session,
				evt.Player2Session,
				evt.Player1Name,
				evt.Player2Name,
				evt.WinnerSession,
				evt.MovesCount,
				evt.Timestamp,
			)
			if err != nil {
				log.Printf("[DB] Failed to insert match: %v", err)
			}

			// Apply points and points_diff adjustments
			if evt.WinnerSession == evt.Player1Session {
				// Player 1 won, Player 2 lost
				_, _ = updateUserWinStmt.Exec(evt.Player1Points, evt.Player1PointsDiff, now, evt.Player1Session)
				_, _ = updateUserLossStmt.Exec(evt.Player2Points, evt.Player2PointsDiff, now, evt.Player2Session)

				_, _ = upsertGameStatsWinStmt.Exec(evt.Player1Session, evt.GameType, evt.Player1Points, evt.Player1PointsDiff, now)
				_, _ = upsertGameStatsLossStmt.Exec(evt.Player2Session, evt.GameType, evt.Player2Points, evt.Player2PointsDiff, now)
			} else if evt.WinnerSession == evt.Player2Session {
				// Player 2 won, Player 1 lost
				_, _ = updateUserWinStmt.Exec(evt.Player2Points, evt.Player2PointsDiff, now, evt.Player2Session)
				_, _ = updateUserLossStmt.Exec(evt.Player1Points, evt.Player1PointsDiff, now, evt.Player1Session)

				_, _ = upsertGameStatsWinStmt.Exec(evt.Player2Session, evt.GameType, evt.Player2Points, evt.Player2PointsDiff, now)
				_, _ = upsertGameStatsLossStmt.Exec(evt.Player1Session, evt.GameType, evt.Player1Points, evt.Player1PointsDiff, now)
			} else {
				// Draw
				_, _ = updateUserDrawStmt.Exec(evt.Player1Points, evt.Player1PointsDiff, now, evt.Player1Session)
				_, _ = updateUserDrawStmt.Exec(evt.Player2Points, evt.Player2PointsDiff, now, evt.Player2Session)

				_, _ = upsertGameStatsDrawStmt.Exec(evt.Player1Session, evt.GameType, evt.Player1Points, evt.Player1PointsDiff, now)
				_, _ = upsertGameStatsDrawStmt.Exec(evt.Player2Session, evt.GameType, evt.Player2Points, evt.Player2PointsDiff, now)
			}
		}
	}

	return tx.Commit()
}

// Close gracefully stops the worker and closes the SQLite database.
func (s *Service) Close() error {
	s.cancel()
	s.wg.Wait()
	return s.db.Close()
}
