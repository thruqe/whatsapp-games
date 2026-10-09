package db

import "time"

// EventType defines the type of interaction event to persist.
type EventType string

const (
	EventRegisterUsername EventType = "register_username"
	EventMatchResult      EventType = "match_result"
)

// InteractionEvent is sent from the Game Logic Agent through the buffered channel.
type InteractionEvent struct {
	Type      EventType
	Timestamp time.Time

	// Fields for username registration / user updates
	SessionID string
	UserJID   string
	Username  string

	// Fields for match results
	GameType       string
	Player1Session string
	Player2Session string
	Player1Name    string
	Player2Name    string
	WinnerSession  string // "sess_p1", "sess_p2", or "draw"
	MovesCount     int

	// Points & Points Diff adjustments
	Player1Points     int
	Player2Points     int
	Player1PointsDiff int
	Player2PointsDiff int
}

// UserProfile holds the persistent player profile.
type UserProfile struct {
	SessionID  string    `json:"session_id"`
	UserJID    string    `json:"user_jid"`
	Username   string    `json:"username"`
	Points     int       `json:"points"`
	Wins       int       `json:"wins"`
	Losses     int       `json:"losses"`
	Draws      int       `json:"draws"`
	PointsDiff int       `json:"points_diff"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// LeaderboardEntry represents ranking entry for public view.
type LeaderboardEntry struct {
	Username   string `json:"username"`
	GameType   string `json:"game_type,omitempty"`
	Points     int    `json:"points"`
	Wins       int    `json:"wins"`
	Losses     int    `json:"losses"`
	Draws      int    `json:"draws"`
	PointsDiff int    `json:"points_diff"`
}
