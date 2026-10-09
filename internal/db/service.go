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
	namesMu  sync.RWMutex
	nameMap  map[string]string // lowercase username -> sessionID
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
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		session_id TEXT PRIMARY KEY,
		user_jid TEXT NOT NULL,
		username TEXT UNIQUE NOT NULL,
		rating INTEGER NOT NULL DEFAULT 1000,
		wins INTEGER NOT NULL DEFAULT 0,
		losses INTEGER NOT NULL DEFAULT 0,
		draws INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_users_rating ON users (rating DESC);
	CREATE INDEX IF NOT EXISTS idx_users_username ON users (username);

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
	_, err := s.db.Exec(schema)
	return err
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
	INSERT INTO users (session_id, user_jid, username, rating, wins, losses, draws, created_at, updated_at)
	VALUES (?, ?, ?, 1000, 0, 0, 0, ?, ?)
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
		SELECT session_id, user_jid, username, rating, wins, losses, draws, created_at, updated_at
		FROM users WHERE session_id = ?`, sessionID)

	var p UserProfile
	err := row.Scan(&p.SessionID, &p.UserJID, &p.Username, &p.Rating, &p.Wins, &p.Losses, &p.Draws, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetLeaderboard returns the top ranked players.
func (s *Service) GetLeaderboard(limit int) ([]LeaderboardEntry, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	rows, err := s.db.Query(`
		SELECT username, rating, wins, losses, draws
		FROM users
		ORDER BY rating DESC, wins DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []LeaderboardEntry
	for rows.Next() {
		var e LeaderboardEntry
		if err := rows.Scan(&e.Username, &e.Rating, &e.Wins, &e.Losses, &e.Draws); err != nil {
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

	updateWinStmt, err := tx.Prepare(`
		UPDATE users SET
			rating = MAX(100, rating + ?),
			wins = wins + 1,
			updated_at = ?
		WHERE session_id = ?
	`)
	if err != nil {
		return fmt.Errorf("prepare win stmt: %w", err)
	}
	defer updateWinStmt.Close()

	updateLossStmt, err := tx.Prepare(`
		UPDATE users SET
			rating = MAX(100, rating + ?),
			losses = losses + 1,
			updated_at = ?
		WHERE session_id = ?
	`)
	if err != nil {
		return fmt.Errorf("prepare loss stmt: %w", err)
	}
	defer updateLossStmt.Close()

	updateDrawStmt, err := tx.Prepare(`
		UPDATE users SET
			rating = MAX(100, rating + ?),
			draws = draws + 1,
			updated_at = ?
		WHERE session_id = ?
	`)
	if err != nil {
		return fmt.Errorf("prepare draw stmt: %w", err)
	}
	defer updateDrawStmt.Close()

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

			// Apply rating and score adjustments
			if evt.WinnerSession == evt.Player1Session {
				_, _ = updateWinStmt.Exec(evt.Player1Delta, now, evt.Player1Session)
				_, _ = updateLossStmt.Exec(evt.Player2Delta, now, evt.Player2Session)
			} else if evt.WinnerSession == evt.Player2Session {
				_, _ = updateWinStmt.Exec(evt.Player2Delta, now, evt.Player2Session)
				_, _ = updateLossStmt.Exec(evt.Player1Delta, now, evt.Player1Session)
			} else {
				// Draw
				_, _ = updateDrawStmt.Exec(evt.Player1Delta, now, evt.Player1Session)
				_, _ = updateDrawStmt.Exec(evt.Player2Delta, now, evt.Player2Session)
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
