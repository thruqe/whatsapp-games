package db

import "time"

// EventType defines the type of interaction event to persist.
type EventType string

const (
	EventRegisterUsername EventType = "register_username"
	EventMatchResult      EventType = "match_result"
	EventRatingUpdate     EventType = "rating_update"
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
	WinnerSession  string
	MovesCount     int

	// Rating adjustments
	Player1Delta int
	Player2Delta int
}

// UserProfile holds the persistent player profile.
type UserProfile struct {
	SessionID string    `json:"session_id"`
	UserJID   string    `json:"user_jid"`
	Username  string    `json:"username"`
	Rating    int       `json:"rating"`
	Wins      int       `json:"wins"`
	Losses    int       `json:"losses"`
	Draws     int       `json:"draws"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// LeaderboardEntry represents ranking entry for public view.
type LeaderboardEntry struct {
	Username string `json:"username"`
	Rating   int    `json:"rating"`
	Wins     int    `json:"wins"`
	Losses   int    `json:"losses"`
	Draws    int    `json:"draws"`
}
