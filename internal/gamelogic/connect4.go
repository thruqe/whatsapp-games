package gamelogic

import "fmt"

const (
	C4Rows = 6
	C4Cols = 7
)

// Connect4Game represents the state of a Connect 4 match.
type Connect4Game struct {
	Board     [C4Rows][C4Cols]string `json:"board"` // row 0 is top, row 5 is bottom
	MoveCount int                    `json:"move_count"`
}

func NewConnect4Game() *Connect4Game {
	return &Connect4Game{}
}

// DropPiece drops a symbol into the selected column (0-6).
// Returns the placed row index or an error if invalid/full.
func (g *Connect4Game) DropPiece(col int, symbol string) (int, error) {
	if col < 0 || col >= C4Cols {
		return -1, fmt.Errorf("column %d out of bounds (0-%d)", col, C4Cols-1)
	}

	// Drop from bottom up (row 5 down to 0)
	for r := C4Rows - 1; r >= 0; r-- {
		if g.Board[r][col] == "" {
			g.Board[r][col] = symbol
			g.MoveCount++
			return r, nil
		}
	}

	return -1, fmt.Errorf("column %d is full", col)
}

// CheckWinner checks for 4-in-a-row in any direction. Returns "R", "Y", "draw", or "".
func (g *Connect4Game) CheckWinner() string {
	// 1. Horizontal
	for r := 0; r < C4Rows; r++ {
		for c := 0; c < C4Cols-3; c++ {
			sym := g.Board[r][c]
			if sym != "" && sym == g.Board[r][c+1] && sym == g.Board[r][c+2] && sym == g.Board[r][c+3] {
				return sym
			}
		}
	}

	// 2. Vertical
	for r := 0; r < C4Rows-3; r++ {
		for c := 0; c < C4Cols; c++ {
			sym := g.Board[r][c]
			if sym != "" && sym == g.Board[r+1][c] && sym == g.Board[r+2][c] && sym == g.Board[r+3][c] {
				return sym
			}
		}
	}

	// 3. Diagonal Down-Right (\)
	for r := 0; r < C4Rows-3; r++ {
		for c := 0; c < C4Cols-3; c++ {
			sym := g.Board[r][c]
			if sym != "" && sym == g.Board[r+1][c+1] && sym == g.Board[r+2][c+2] && sym == g.Board[r+3][c+3] {
				return sym
			}
		}
	}

	// 4. Diagonal Up-Right (/)
	for r := 3; r < C4Rows; r++ {
		for c := 0; c < C4Cols-3; c++ {
			sym := g.Board[r][c]
			if sym != "" && sym == g.Board[r-1][c+1] && sym == g.Board[r-2][c+2] && sym == g.Board[r-3][c+3] {
				return sym
			}
		}
	}

	if g.MoveCount >= C4Rows*C4Cols {
		return "draw"
	}

	return ""
}
