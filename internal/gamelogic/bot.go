package gamelogic

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
)

var (
	botNameMu  sync.Mutex
	usedNames  = make(map[string]bool)

	esotericPrefixes = []string{
		"Quantum", "Xenon", "Axiom", "Entropy", "Neuro",
		"Eigen", "Chrono", "Sublime", "Obsidian", "Abyssal",
		"Stochastic", "Soliton", "NonEuclidean", "Isotope", "Decoherence",
		"Hyperion", "Celestial", "Orthogonal", "Markov", "Spectral",
		"Synaptic", "Voidform", "Kinetik", "Parallax", "Aether",
	}

	esotericNouns = []string{
		"Sovereign", "Archon", "Tesseract", "Synthesizer", "Monolith",
		"Automaton", "Singularity", "Spectre", "Protocol", "Waveguide",
		"Recursion", "Phosphor", "Drifter", "Manifold", "Anomalist",
		"Valkyrie", "Oracle", "Heuristic", "Stratagem", "Nexus",
		"Apex", "Chimera", "Resonator", "Paradox", "Zephyr",
	}

	esotericSuffixes = []string{
		".exe", ".ai", "_0x7F", "_Prime", "::Null", "_v4",
		"_Omni", "_Delta", "_Theta", "_Zero", "_IV", "_Omega",
		"_Void", ".core", "_Sigma",
	}
)

// GenerateUniqueBotName generates an esoteric, non-traditional player name.
func GenerateUniqueBotName() string {
	botNameMu.Lock()
	defer botNameMu.Unlock()

	for attempts := 0; attempts < 100; attempts++ {
		pIdx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(esotericPrefixes))))
		nIdx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(esotericNouns))))
		sIdx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(esotericSuffixes))))

		prefix := esotericPrefixes[pIdx.Int64()]
		noun := esotericNouns[nIdx.Int64()]
		suffix := esotericSuffixes[sIdx.Int64()]

		name := fmt.Sprintf("%s_%s%s", prefix, noun, suffix)
		if !usedNames[name] {
			usedNames[name] = true
			return name
		}
	}

	// Fallback unique hex suffix
	r, _ := rand.Int(rand.Reader, big.NewInt(0xFFFF))
	fallback := fmt.Sprintf("Axiom_Zero_0x%04X", r.Int64())
	usedNames[fallback] = true
	return fallback
}

// SelectTTTMove chooses an intelligent move for Tic-Tac-Toe.
func SelectTTTMove(board [9]string, botSymbol, humanSymbol string) int {
	// 1. Can bot win in 1 move?
	for i := 0; i < 9; i++ {
		if board[i] == "" {
			boardCopy := board
			boardCopy[i] = botSymbol
			if checkTTTWinner(boardCopy) == botSymbol {
				return i
			}
		}
	}

	// 2. Can human win in 1 move? Block it!
	for i := 0; i < 9; i++ {
		if board[i] == "" {
			boardCopy := board
			boardCopy[i] = humanSymbol
			if checkTTTWinner(boardCopy) == humanSymbol {
				return i
			}
		}
	}

	// 3. Take center if available
	if board[4] == "" {
		return 4
	}

	// 4. Take corners (0, 2, 6, 8)
	corners := []int{0, 2, 6, 8}
	var openCorners []int
	for _, c := range corners {
		if board[c] == "" {
			openCorners = append(openCorners, c)
		}
	}
	if len(openCorners) > 0 {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(openCorners))))
		return openCorners[idx.Int64()]
	}

	// 5. Take any remaining open cell
	var openCells []int
	for i := 0; i < 9; i++ {
		if board[i] == "" {
			openCells = append(openCells, i)
		}
	}
	if len(openCells) > 0 {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(openCells))))
		return openCells[idx.Int64()]
	}

	return -1
}

func checkTTTWinner(board [9]string) string {
	lines := [][3]int{
		{0, 1, 2}, {3, 4, 5}, {6, 7, 8}, // Rows
		{0, 3, 6}, {1, 4, 7}, {2, 5, 8}, // Columns
		{0, 4, 8}, {2, 4, 6}, // Diagonals
	}
	for _, l := range lines {
		if board[l[0]] != "" && board[l[0]] == board[l[1]] && board[l[1]] == board[l[2]] {
			return board[l[0]]
		}
	}
	return ""
}

// SelectC4Move chooses an intelligent move for Connect 4.
func SelectC4Move(board [C4Rows][C4Cols]string, botSymbol, humanSymbol string) int {
	// 1. Can bot win in 1 drop?
	for col := 0; col < C4Cols; col++ {
		if canDropC4(board, col) {
			bCopy := board
			dropC4Piece(&bCopy, col, botSymbol)
			if checkC4Winner(bCopy) == botSymbol {
				return col
			}
		}
	}

	// 2. Can human win in 1 drop? Block it!
	for col := 0; col < C4Cols; col++ {
		if canDropC4(board, col) {
			bCopy := board
			dropC4Piece(&bCopy, col, humanSymbol)
			if checkC4Winner(bCopy) == humanSymbol {
				return col
			}
		}
	}

	// 3. Evaluate safe moves (avoid dropping if it sets up opponent win above)
	priorityCols := []int{3, 2, 4, 1, 5, 0, 6}
	var safeCols []int

	for _, col := range priorityCols {
		if canDropC4(board, col) {
			bCopy := board
			row := dropC4Piece(&bCopy, col, botSymbol)
			// Check if row above gives opponent immediate win
			if row > 0 {
				bCopyAbove := bCopy
				bCopyAbove[row-1][col] = humanSymbol
				if checkC4Winner(bCopyAbove) == humanSymbol {
					continue // Blunder! Don't play here
				}
			}
			safeCols = append(safeCols, col)
		}
	}

	if len(safeCols) > 0 {
		return safeCols[0] // Highest priority safe column
	}

	// 4. Fallback to any valid column
	for _, col := range priorityCols {
		if canDropC4(board, col) {
			return col
		}
	}

	return -1
}

func canDropC4(board [C4Rows][C4Cols]string, col int) bool {
	return board[0][col] == ""
}

func dropC4Piece(board *[C4Rows][C4Cols]string, col int, symbol string) int {
	for r := C4Rows - 1; r >= 0; r-- {
		if board[r][col] == "" {
			board[r][col] = symbol
			return r
		}
	}
	return -1
}

func checkC4Winner(board [C4Rows][C4Cols]string) string {
	// Horizontal
	for r := 0; r < C4Rows; r++ {
		for c := 0; c < C4Cols-3; c++ {
			sym := board[r][c]
			if sym != "" && sym == board[r][c+1] && sym == board[r][c+2] && sym == board[r][c+3] {
				return sym
			}
		}
	}
	// Vertical
	for r := 0; r < C4Rows-3; r++ {
		for c := 0; c < C4Cols; c++ {
			sym := board[r][c]
			if sym != "" && sym == board[r+1][c] && sym == board[r+2][c] && sym == board[r+3][c] {
				return sym
			}
		}
	}
	// Diagonal Down-Right
	for r := 0; r < C4Rows-3; r++ {
		for c := 0; c < C4Cols-3; c++ {
			sym := board[r][c]
			if sym != "" && sym == board[r+1][c+1] && sym == board[r+2][c+2] && sym == board[r+3][c+3] {
				return sym
			}
		}
	}
	// Diagonal Up-Right
	for r := 3; r < C4Rows; r++ {
		for c := 0; c < C4Cols-3; c++ {
			sym := board[r][c]
			if sym != "" && sym == board[r-1][c+1] && sym == board[r-2][c+2] && sym == board[r-3][c+3] {
				return sym
			}
		}
	}
	return ""
}
