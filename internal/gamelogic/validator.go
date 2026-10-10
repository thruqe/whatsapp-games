package gamelogic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"
)

// WordValidator validates English words using an in-memory cache, an extensive
// local dictionary, and multiple external dictionary & word-checking APIs.
type WordValidator struct {
	client     *http.Client
	cache      sync.Map // map[string]bool
	localWords map[string]bool
	timeout    time.Duration
}

// Global default validator instance used across game sessions.
var defaultValidator *WordValidator
var defaultValidatorOnce sync.Once

// GetDefaultValidator returns the singleton WordValidator.
func GetDefaultValidator() *WordValidator {
	defaultValidatorOnce.Do(func() {
		defaultValidator = NewWordValidator()
	})
	return defaultValidator
}

// NewWordValidator constructs a configured WordValidator with local dictionary and HTTP client.
func NewWordValidator() *WordValidator {
	v := &WordValidator{
		client: &http.Client{
			Timeout: 3 * time.Second,
		},
		timeout:    3500 * time.Millisecond,
		localWords: make(map[string]bool),
	}

	// Populate local dictionary with words from dictionaryByLetter
	for _, words := range dictionaryByLetter {
		for _, w := range words {
			v.localWords[strings.ToLower(w)] = true
		}
	}

	// Populate additional extensive local word list
	for _, w := range extendedCommonWords {
		v.localWords[strings.ToLower(w)] = true
	}

	return v
}

// ValidateWord checks if a word is an authentic English word.
func (v *WordValidator) ValidateWord(ctx context.Context, rawWord string) bool {
	word := strings.ToLower(strings.TrimSpace(rawWord))
	if len(word) < 2 {
		return false
	}

	for _, r := range word {
		if !unicode.IsLetter(r) {
			return false
		}
	}

	// 1. In-memory cache lookup
	if val, ok := v.cache.Load(word); ok {
		return val.(bool)
	}

	// 2. Embedded local dictionary lookup (instant 0ms response)
	if v.localWords[word] {
		v.cache.Store(word, true)
		return true
	}

	// 3. Multi-API Concurrent Validation
	ctx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

	type apiResult struct {
		name    string
		isValid bool
		err     error
	}

	apis := []struct {
		name string
		fn   func(context.Context, string) (bool, error)
	}{
		{"datamuse", v.checkDatamuse},
		{"wiktionary_query", v.checkWiktionaryQuery},
		{"wiktionary_rest", v.checkWiktionaryREST},
		{"freedictionary", v.checkFreeDictionaryAPI},
	}

	resChan := make(chan apiResult, len(apis))

	for _, api := range apis {
		go func(name string, checkFn func(context.Context, string) (bool, error)) {
			valid, err := checkFn(ctx, word)
			resChan <- apiResult{name: name, isValid: valid, err: err}
		}(api.name, api.fn)
	}

	var confirmedNegativeCount int
	var errorCount int

	for i := 0; i < len(apis); i++ {
		select {
		case res := <-resChan:
			if res.err == nil {
				if res.isValid {
					// Word validated as real by external API!
					v.cache.Store(word, true)
					return true
				}
				confirmedNegativeCount++
			} else {
				errorCount++
			}
		case <-ctx.Done():
			// Timeout reached. Fall back to local dictionary.
			return v.localWords[word]
		}
	}

	// If at least one API successfully confirmed the word is NOT in the dictionary,
	// and none confirmed it, mark as invalid.
	if confirmedNegativeCount > 0 {
		v.cache.Store(word, false)
		return false
	}

	// If all APIs encountered network errors or were unreachable, fallback to local words.
	return v.localWords[word]
}

// 1. Datamuse API (https://api.datamuse.com/words?sp=<word>&max=1)
func (v *WordValidator) checkDatamuse(ctx context.Context, word string) (bool, error) {
	reqURL := fmt.Sprintf("https://api.datamuse.com/words?sp=%s&max=1", url.QueryEscape(word))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", "WhatsAppGamesBot/1.0")

	resp, err := v.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("datamuse status %d", resp.StatusCode)
	}

	var results []struct {
		Word  string `json:"word"`
		Score int    `json:"score"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return false, err
	}

	if len(results) > 0 && strings.EqualFold(results[0].Word, word) {
		return true, nil
	}
	return false, nil
}

// 2. Wikimedia / Wiktionary Action Query API (https://en.wiktionary.org/w/api.php)
func (v *WordValidator) checkWiktionaryQuery(ctx context.Context, word string) (bool, error) {
	reqURL := fmt.Sprintf("https://en.wiktionary.org/w/api.php?action=query&titles=%s&format=json", url.QueryEscape(word))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", "WhatsAppGamesBot/1.0 (https://github.com/thruqe/whatsapp-games)")

	resp, err := v.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("wiktionary query status %d", resp.StatusCode)
	}

	var data struct {
		Query struct {
			Pages map[string]struct {
				PageID  int    `json:"pageid"`
				Missing string `json:"missing"`
			} `json:"pages"`
		} `json:"query"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return false, err
	}

	for _, page := range data.Query.Pages {
		if page.PageID > 0 && page.Missing == "" {
			return true, nil
		}
	}
	return false, nil
}

// 3. Wikimedia / Wiktionary REST Definition API (https://en.wiktionary.org/api/rest_v1/page/definition/<word>)
func (v *WordValidator) checkWiktionaryREST(ctx context.Context, word string) (bool, error) {
	reqURL := fmt.Sprintf("https://en.wiktionary.org/api/rest_v1/page/definition/%s", url.PathEscape(word))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", "WhatsAppGamesBot/1.0 (https://github.com/thruqe/whatsapp-games)")

	resp, err := v.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	} else if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return false, fmt.Errorf("wiktionary rest status %d", resp.StatusCode)
}

// 4. Free Dictionary API (https://api.dictionaryapi.dev/api/v2/entries/en/<word>)
func (v *WordValidator) checkFreeDictionaryAPI(ctx context.Context, word string) (bool, error) {
	reqURL := fmt.Sprintf("https://api.dictionaryapi.dev/api/v2/entries/en/%s", url.PathEscape(word))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", "WhatsAppGamesBot/1.0")

	resp, err := v.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		// Valid dictionary entry found
		io.Copy(io.Discard, resp.Body)
		return true, nil
	} else if resp.StatusCode == http.StatusNotFound {
		io.Copy(io.Discard, resp.Body)
		return false, nil
	}
	return false, fmt.Errorf("freedictionary status %d", resp.StatusCode)
}
