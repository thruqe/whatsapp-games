package gamelogic

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"unicode"
)

// WCGWordEntry tracks a played word in the chain.
type WCGWordEntry struct {
	Word   string `json:"word"`
	Player string `json:"player"`
	Symbol string `json:"symbol"`
	Points int    `json:"points"`
}

// WCGGame represents a Word Chain Game match state.
type WCGGame struct {
	RequiredLetter string          `json:"required_letter"`
	MinLength      int             `json:"min_length"`
	Chain          []WCGWordEntry  `json:"chain"`
	UsedWords      map[string]bool `json:"used_words"`
	P1Score        int             `json:"p1_score"`
	P2Score        int             `json:"p2_score"`
	P1Strikes      int             `json:"p1_strikes"`
	P2Strikes      int             `json:"p2_strikes"`
	MoveCount      int             `json:"move_count"`
	MaxRounds      int             `json:"max_rounds"`
}

func NewWCGGame() *WCGGame {
	startingLetters := []string{"A", "B", "C", "D", "E", "F", "G", "H", "M", "P", "R", "S", "T", "W"}
	idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(startingLetters))))

	return &WCGGame{
		RequiredLetter: startingLetters[idx.Int64()],
		MinLength:      3,
		Chain:          make([]WCGWordEntry, 0),
		UsedWords:      make(map[string]bool),
		MaxRounds:      16, // 8 words per player
	}
}

// SubmitWord processes a word submission.
// Returns points awarded, winner symbol ("P1", "P2", "draw", or ""), and error if invalid.
func (g *WCGGame) SubmitWord(rawWord string, playerSymbol string, playerName string) (int, string, error) {
	word := strings.ToLower(strings.TrimSpace(rawWord))

	// 1. Check length
	if len(word) < g.MinLength {
		g.addStrike(playerSymbol)
		winner := g.checkStrikeWinner()
		return 0, winner, fmt.Errorf("word must be at least %d letters long (strike %d/3)", g.MinLength, g.getStrikes(playerSymbol))
	}

	// 2. Check characters (alphabetical only)
	for _, r := range word {
		if !unicode.IsLetter(r) {
			g.addStrike(playerSymbol)
			winner := g.checkStrikeWinner()
			return 0, winner, fmt.Errorf("word must contain only letters (strike %d/3)", g.getStrikes(playerSymbol))
		}
	}

	// 3. Check starting letter
	firstChar := strings.ToUpper(string(word[0]))
	if firstChar != g.RequiredLetter {
		g.addStrike(playerSymbol)
		winner := g.checkStrikeWinner()
		return 0, winner, fmt.Errorf("word must start with letter '%s' (strike %d/3)", g.RequiredLetter, g.getStrikes(playerSymbol))
	}

	// 4. Check if already used
	if g.UsedWords[word] {
		g.addStrike(playerSymbol)
		winner := g.checkStrikeWinner()
		return 0, winner, fmt.Errorf("word '%s' was already played this match (strike %d/3)", word, g.getStrikes(playerSymbol))
	}

	// 5. Check dictionary validity
	if !IsValidEnglishWord(word) {
		g.addStrike(playerSymbol)
		winner := g.checkStrikeWinner()
		return 0, winner, fmt.Errorf("'%s' is not in the game dictionary (strike %d/3)", word, g.getStrikes(playerSymbol))
	}

	// Valid move!
	wordPoints := len(word) * 5
	g.UsedWords[word] = true
	g.MoveCount++

	if playerSymbol == "P1" {
		g.P1Score += wordPoints
	} else {
		g.P2Score += wordPoints
	}

	g.Chain = append(g.Chain, WCGWordEntry{
		Word:   word,
		Player: playerName,
		Symbol: playerSymbol,
		Points: wordPoints,
	})

	// Next required letter is the last character of the word
	lastChar := strings.ToUpper(string(word[len(word)-1]))
	g.RequiredLetter = lastChar

	// Check if match reached max turns
	if g.MoveCount >= g.MaxRounds {
		if g.P1Score > g.P2Score {
			return wordPoints, "P1", nil
		} else if g.P2Score > g.P1Score {
			return wordPoints, "P2", nil
		}
		return wordPoints, "draw", nil
	}

	return wordPoints, "", nil
}

func (g *WCGGame) addStrike(symbol string) {
	if symbol == "P1" {
		g.P1Strikes++
	} else {
		g.P2Strikes++
	}
}

func (g *WCGGame) getStrikes(symbol string) int {
	if symbol == "P1" {
		return g.P1Strikes
	}
	return g.P2Strikes
}

func (g *WCGGame) checkStrikeWinner() string {
	if g.P1Strikes >= 3 {
		return "P2"
	}
	if g.P2Strikes >= 3 {
		return "P1"
	}
	return ""
}

// SelectWCGMove picks a valid unused word starting with the required letter for the automated bot.
func SelectWCGMove(reqLetter string, usedWords map[string]bool) string {
	reqLetter = strings.ToLower(reqLetter)
	candidates, ok := dictionaryByLetter[reqLetter]
	if !ok || len(candidates) == 0 {
		return ""
	}

	var valid []string
	for _, w := range candidates {
		if !usedWords[w] {
			valid = append(valid, w)
		}
	}

	if len(valid) == 0 {
		return ""
	}

	idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(valid))))
	return valid[idx.Int64()]
}

// IsValidEnglishWord checks if a word exists in the dictionary.
func IsValidEnglishWord(word string) bool {
	word = strings.ToLower(strings.TrimSpace(word))
	firstChar := string(word[0])
	bucket, ok := dictionaryByLetter[firstChar]
	if !ok {
		return false
	}
	for _, w := range bucket {
		if w == word {
			return true
		}
	}
	return false
}

// Extensive dictionary partitioned by starting letter for instant lookups
var dictionaryByLetter = map[string][]string{
	"a": {"apple", "anchor", "arrow", "angel", "acorn", "apron", "amber", "armor", "arena", "audio", "atlas", "axiom", "amber", "alpha", "asset", "alarm", "actor", "alert", "album", "agent", "alias", "alter", "alive", "alone", "among", "amuse", "angel", "anger", "angle", "ankle", "apply", "arise", "aside", "audit", "avail", "avoid", "await", "awake", "award", "aware"},
	"b": {"banana", "bridge", "bottle", "butter", "basket", "beacon", "bishop", "branch", "breeze", "banner", "blazer", "blouse", "bonnet", "border", "bronze", "bubble", "bullet", "bundle", "burden", "button", "barrel", "batter", "beauty", "beaver", "beetle", "belief", "belong", "beside", "beyond", "binary", "bitter", "blanket", "blossom", "boulder", "bravery", "breathe", "builder", "bulletin", "burglar", "barrier"},
	"c": {"castle", "candle", "camera", "canyon", "carpet", "carrot", "cattle", "celery", "cereal", "chimney", "circle", "circus", "citrus", "clover", "coffee", "collar", "column", "comedy", "comet", "compass", "copper", "cotton", "cousin", "cradle", "crater", "credit", "cricket", "custom", "crystal", "cushion", "canary", "cannon", "canvas", "captain", "carbon", "careful", "carrier", "cascade", "ceiling", "cellar"},
	"d": {"dragon", "desert", "dolphin", "diamond", "dinner", "doctor", "donkey", "dragon", "drawer", "driver", "dancer", "danger", "decade", "degree", "demand", "denial", "deputy", "design", "desire", "detail", "device", "differ", "dinner", "direct", "domain", "double", "dragon", "drawer", "driver", "dugout", "durable", "dynamic", "dagger", "daisy", "damage", "dancer", "daring", "darkness", "daylight", "dazzle"},
	"e": {"eagle", "earth", "engine", "echo", "edge", "effort", "elbow", "elder", "element", "elephant", "ember", "emerald", "empire", "energy", "engine", "entity", "entry", "envelope", "epoch", "equal", "equity", "eraser", "escape", "estate", "ether", "ethics", "event", "evidence", "exact", "exile", "expert", "export", "extent", "eyeball", "eager", "earnest", "easel", "eclipse", "economy", "edition"},
	"f": {"forest", "falcon", "feather", "fender", "finger", "flavor", "flower", "flute", "forest", "fossil", "fountain", "freeze", "fridge", "friend", "frog", "frost", "fruit", "furnace", "future", "fabric", "factor", "factory", "family", "famous", "farmer", "father", "feature", "female", "filter", "finite", "flame", "flight", "float", "flock", "fluid", "focus", "folder", "follow", "foster", "fortune"},
	"g": {"garden", "galaxy", "garlic", "gemini", "gentle", "geyser", "giant", "ginger", "giraffe", "glacier", "glance", "glider", "global", "globe", "glory", "glove", "goblet", "golden", "gospel", "govern", "grace", "grade", "grain", "grand", "grape", "graph", "grass", "grave", "gravity", "greed", "green", "greet", "grief", "grill", "grind", "groove", "ground", "growth", "guard", "guitar"},
	"h": {"harbor", "hammer", "handle", "hanger", "happen", "harbor", "harvest", "hazard", "header", "health", "hearth", "heaven", "helmet", "helper", "herald", "hermit", "heroic", "hidden", "highway", "hollow", "honest", "honey", "honor", "horizon", "horror", "horse", "hotel", "house", "hover", "human", "humble", "humor", "hunger", "hunter", "hybrid", "hyper", "habit", "hail", "half", "halo"},
	"i": {"island", "iceberg", "icon", "idea", "ideal", "idiom", "idle", "igloo", "image", "impact", "import", "impose", "income", "index", "indigo", "infant", "inform", "inflow", "inland", "inlet", "inner", "input", "insect", "inside", "insight", "intact", "intent", "invest", "invite", "island", "issue", "ivory", "irony", "item", "itch", "isolate", "income", "intake", "invent", "invoke"},
	"j": {"jungle", "jacket", "jaguar", "jargon", "javelin", "jealous", "jersey", "jester", "jewel", "jigsaw", "jingle", "jockey", "jovial", "judge", "juice", "jumbo", "jumper", "jungle", "junior", "juror", "justice", "justify", "jazz", "join", "joke", "jolt", "journal", "journey", "joyful", "jubilee"},
	"k": {"knight", "kettle", "keypad", "kidnap", "kidney", "kimono", "kindle", "kinetic", "kingdom", "kiosk", "kitten", "knapsack", "knave", "knead", "knife", "knight", "knocker", "knoll", "knot", "know", "koala", "kudos", "kale", "karma", "kayak", "keeper", "kernel", "ketchup", "khaki", "kickoff"},
	"l": {"lemon", "ladder", "lagoon", "lantern", "laptop", "laurel", "lavender", "lawyer", "leader", "league", "legacy", "legend", "lemon", "leopard", "lesson", "letter", "lever", "liberty", "lichen", "lightning", "lilac", "linear", "linger", "liquid", "listen", "lizard", "locket", "locust", "lodge", "logic", "lotus", "lounge", "lumber", "lunar", "luxury", "labor", "lace", "lady", "lake", "lamb"},
	"m": {"monkey", "magnet", "magpie", "maiden", "mango", "manner", "manor", "mantle", "maple", "marble", "margin", "marine", "market", "marmot", "matrix", "meadow", "medal", "melon", "mentor", "meteor", "middle", "miller", "mineral", "mirror", "mission", "mobile", "model", "modern", "modest", "moment", "monarch", "monkey", "monument", "morning", "mortar", "mosaic", "motion", "motive", "mountain", "museum"},
	"n": {"night", "nature", "nebula", "nectar", "needle", "nephew", "nestle", "network", "neutron", "nickel", "night", "nimble", "nitrogen", "noble", "nomad", "noodle", "normal", "notice", "novel", "nozzle", "nucleus", "nugget", "number", "nursery", "nutmeg", "nylon", "nail", "name", "narrow", "native", "navy", "near", "neat", "neck", "need", "neon", "nerve", "nest", "news", "next"},
	"o": {"orange", "oasis", "object", "oboe", "ocean", "octave", "octane", "octopus", "office", "olive", "omelet", "onion", "online", "opera", "optics", "orange", "orbit", "orchard", "orchid", "organ", "orient", "origin", "osprey", "outfit", "outlet", "output", "outrun", "oxford", "oxygen", "oyster", "oak", "oar", "oath", "oats", "obey", "occur", "odd", "odor", "off", "often"},
	"p": {"planet", "palace", "palette", "panther", "papaya", "parade", "parcel", "parrot", "parsnip", "pastor", "pasture", "patent", "patrol", "patron", "pebble", "pelican", "pencil", "pendant", "penguin", "pepper", "period", "person", "phantom", "phoenix", "pickle", "picnic", "pigeon", "pillar", "pillow", "pilot", "pinball", "pioneer", "pirate", "pistol", "piston", "planet", "plasma", "plaster", "plateau", "platter"},
	"q": {"quartz", "quarry", "quarter", "queen", "quench", "query", "quest", "quiver", "quorum", "quota", "quote", "quack", "quail", "quaint", "quake", "qualm", "quantum", "quark", "quarrel", "quartet"},
	"r": {"rabbit", "radar", "radiant", "radius", "rafter", "rainbow", "raptor", "raven", "reactor", "realm", "rebel", "record", "refuge", "regal", "region", "relic", "remedy", "remote", "render", "republic", "rescue", "resin", "resort", "result", "retail", "rhythm", "ribbon", "riddle", "rider", "ridge", "rifle", "ripple", "ritual", "river", "rocket", "rooster", "rotate", "rotor", "rover", "ruby"},
	"s": {"silver", "saddle", "safari", "salmon", "saloon", "sample", "sandal", "sapphire", "satchel", "satellite", "saucer", "savage", "saving", "scalar", "scanner", "scarlet", "scatter", "scenic", "scepter", "scholar", "scooter", "scout", "scramble", "screen", "script", "sculpt", "season", "second", "secret", "sector", "seldom", "senate", "senior", "sensor", "serpent", "servant", "settle", "shadow", "shifter", "silver"},
	"t": {"tiger", "table", "tablet", "tactics", "talent", "tamale", "tandem", "tangle", "tanker", "target", "tariff", "tavern", "teller", "temper", "temple", "tenant", "tender", "tenor", "tension", "terrace", "terrain", "terror", "theory", "thesis", "thistle", "ticket", "timber", "tinsel", "tissue", "toaster", "tomato", "tongue", "topaz", "tornado", "torpedo", "torrent", "tourist", "tractor", "traffic", "tragedy"},
	"u": {"umbrella", "umpire", "uncle", "unicorn", "uniform", "unique", "unit", "universe", "update", "uphold", "uplink", "upload", "upright", "uproar", "uproot", "upset", "upward", "uranium", "urban", "urchin", "urgent", "utility", "utopia", "utter", "ugly", "ultra", "under", "undo", "unfit", "unite"},
	"v": {"violet", "vacuum", "valley", "valve", "vanilla", "vapor", "vector", "velvet", "vendor", "venture", "venue", "verdict", "vessel", "veteran", "vibrant", "victim", "victor", "village", "vintage", "violet", "violin", "viper", "virtue", "vision", "visual", "vortex", "voyage", "vulture", "valid", "valor", "value", "vampire", "vanish", "vapor", "vault", "vault", "veil", "vein", "velocity", "verse"},
	"w": {"winter", "waffle", "walnut", "walrus", "wander", "warden", "warfare", "warrior", "wasp", "water", "weasel", "weapon", "weaver", "weekly", "welcome", "western", "whale", "wheat", "whistle", "widget", "willow", "window", "winner", "winter", "wisdom", "wizard", "wombat", "wonder", "woodland", "worker", "world", "worship", "wreath", "writer", "wader", "wagon", "waist", "waiter", "waiver", "walker"},
	"x": {"xenon", "xray", "xylophone", "xenial", "xeric"},
	"y": {"yellow", "yacht", "yard", "yarn", "year", "yeast", "yodel", "yoga", "yogurt", "yolk", "young", "youth", "yucca", "yawn", "yield", "yonder"},
	"z": {"zebra", "zephyr", "zero", "zest", "zigzag", "zinc", "zipper", "zircon", "zodiac", "zombie", "zone", "zoology", "zoom"},
}
