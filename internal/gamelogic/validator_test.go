package gamelogic

import (
	"context"
	"testing"
)

func TestWordValidator(t *testing.T) {
	v := GetDefaultValidator()
	ctx := context.Background()

	// 1. Basic structural checks
	if v.ValidateWord(ctx, "") {
		t.Errorf("expected empty string to be invalid")
	}
	if v.ValidateWord(ctx, "a") {
		t.Errorf("expected 1-letter string to be invalid")
	}
	if v.ValidateWord(ctx, "12345") {
		t.Errorf("expected digits to be invalid")
	}
	if v.ValidateWord(ctx, "hello!") {
		t.Errorf("expected punctuation to be invalid")
	}

	// 2. Embedded local words (instant lookup)
	localWords := []string{"apple", "bridge", "castle", "dragon", "elephant", "forest", "galaxy"}
	for _, w := range localWords {
		if !v.ValidateWord(ctx, w) {
			t.Errorf("expected local word %q to be valid", w)
		}
	}

	// 3. Rare or advanced real English words validated via dictionary APIs
	// (These are authentic words that may not be in the original 40-word buckets)
	apiWords := []string{"serendipity", "algorithm", "kaleidoscope"}
	for _, w := range apiWords {
		if !v.ValidateWord(ctx, w) {
			t.Errorf("expected real English word %q to be valid via dictionary APIs", w)
		}
	}

	// 4. Verify cache hit (second lookup should be served from memory cache)
	if !v.ValidateWord(ctx, "serendipity") {
		t.Errorf("expected cached word 'serendipity' to be valid")
	}

	// 5. Total nonsense / gibberish words should fail validation
	fakeWords := []string{"zxqzqxw", "qpxwvbmj", "qwertyuiopasdf"}
	for _, w := range fakeWords {
		if v.ValidateWord(ctx, w) {
			t.Errorf("expected fake word %q to be invalid", w)
		}
	}
}
