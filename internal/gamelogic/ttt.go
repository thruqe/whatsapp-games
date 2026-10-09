package gamelogic

import "fmt"

// TTTGame represents the state of a Tic-Tac-Toe match.
type TTTGame struct {
	Board     [9]string `json:"board"` // 0-8: "", "X", "O"
	MoveCount int       `json:"move_count"`
}

func NewTTTGame() *TTTGame {
	return &TTTGame{}
}

// MakeMove attempts to place a symbol at position index (0-8).
func (g *TTTGame) MakeMove(pos int, symbol string) error {
	if pos < 0 || pos > 8 {
		return fmt.Errorf("position %d out of bounds (0-8)", pos)
	}
	if g.Board[pos] != "" {
		return fmt.Errorf("cell %d is already occupied", pos)
	}
	g.Board[pos] = symbol
	g.MoveCount++
	return nil
}

// CheckWinner checks for a 3-in-a-row winner. Returns "X", "O", "draw", or "".
func (g *TTTGame) CheckWinner() string {
	lines := [][3]int{
		{0, 1, 2}, {3, 4, 5}, {6, 7, 8}, // Rows
		{0, 3, 6}, {1, 4, 7}, {2, 5, 8}, // Columns
		{0, 4, 8}, {2, 4, 6}, // Diagonals
	}

	for _, l := range lines {
		if g.Board[l[0]] != "" && g.Board[l[0]] == g.Board[l[1]] && g.Board[l[1]] == g.Board[l[2]] {
			return g.Board[l[0]]
		}
	}

	if g.MoveCount >= 9 {
		return "draw"
	}

	return ""
}
