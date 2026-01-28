package matcher

import (
	"regexp"
	"strings"
)

var (
	apostropheRe   = regexp.MustCompile(`[''']`)
	nonAlphaRe     = regexp.MustCompile(`[^a-z0-9]+`)
	numberRe       = regexp.MustCompile(`\d+(\.\d+)?`)
	wsRe           = regexp.MustCompile(`\s+`)
	resolutionDate = regexp.MustCompile(`(?i)(?:by|before|on)\s+(January|February|March|April|May|June|July|August|September|October|November|December)\s+(\d{1,2}),?\s+(\d{4})`)
)

var stopwords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true,
	"at": true, "be": true, "before": true, "by": true, "does": true,
	"for": true, "from": true, "if": true, "in": true, "into": true,
	"is": true, "it": true, "of": true, "on": true, "or": true,
	"the": true, "this": true, "to": true, "was": true, "were": true,
	"will": true, "with": true,
}

// Tokenize splits text into meaningful tokens, removing stopwords and short words.
func Tokenize(text string) []string {
	if text == "" {
		return nil
	}
	cleaned := strings.ToLower(text)
	cleaned = apostropheRe.ReplaceAllString(cleaned, "")
	cleaned = nonAlphaRe.ReplaceAllString(cleaned, " ")
	cleaned = strings.TrimSpace(cleaned)

	parts := wsRe.Split(cleaned, -1)
	var tokens []string
	for _, t := range parts {
		if t == "" {
			continue
		}
		isNum := true
		for _, c := range t {
			if c < '0' || c > '9' {
				isNum = false
				break
			}
		}
		if len(t) <= 2 && !isNum {
			continue
		}
		if stopwords[t] {
			continue
		}
		tokens = append(tokens, t)
	}
	return tokens
}

// TokenSet returns a set of tokens from text.
func TokenSet(text string) map[string]bool {
	set := make(map[string]bool)
	for _, t := range Tokenize(text) {
		set[t] = true
	}
	return set
}

// BigramSet returns a set of character bigrams from text.
func BigramSet(text string) map[string]bool {
	if text == "" {
		return make(map[string]bool)
	}
	cleaned := strings.ToLower(text)
	cleaned = apostropheRe.ReplaceAllString(cleaned, "")
	cleaned = nonAlphaRe.ReplaceAllString(cleaned, "")
	if len(cleaned) < 2 {
		return make(map[string]bool)
	}

	set := make(map[string]bool)
	for i := 0; i < len(cleaned)-1; i++ {
		set[cleaned[i:i+2]] = true
	}
	return set
}

// DiceCoefficient computes the Dice coefficient between two sets.
func DiceCoefficient(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	overlap := 0
	for k := range a {
		if b[k] {
			overlap++
		}
	}
	return float64(2*overlap) / float64(len(a)+len(b))
}

// ExtractNumbers extracts all numbers from text.
func ExtractNumbers(text string) map[string]bool {
	if text == "" {
		return make(map[string]bool)
	}
	matches := numberRe.FindAllString(text, -1)
	set := make(map[string]bool)
	for _, m := range matches {
		set[m] = true
	}
	return set
}

// ExtractYears extracts 4-digit years (1900-2100) from text.
func ExtractYears(text string) map[string]bool {
	numbers := ExtractNumbers(text)
	years := make(map[string]bool)
	for n := range numbers {
		if len(n) != 4 {
			continue
		}
		year := 0
		valid := true
		for _, c := range n {
			if c < '0' || c > '9' {
				valid = false
				break
			}
			year = year*10 + int(c-'0')
		}
		if valid && year >= 1900 && year <= 2100 {
			years[n] = true
		}
	}
	return years
}

// SetsEqual returns true if sets a and b contain exactly the same elements.
func SetsEqual(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

// HasOverlap returns true if sets a and b share at least one element.
func HasOverlap(a, b map[string]bool) bool {
	for k := range a {
		if b[k] {
			return true
		}
	}
	return false
}

// monthNames maps month name variants to canonical lowercase form.
var monthNames = map[string]string{
	"january": "january", "jan": "january",
	"february": "february", "feb": "february",
	"march": "march", "mar": "march",
	"april": "april", "apr": "april",
	"may": "may",
	"june": "june", "jun": "june",
	"july": "july", "jul": "july",
	"august": "august", "aug": "august",
	"september": "september", "sep": "september", "sept": "september",
	"october": "october", "oct": "october",
	"november": "november", "nov": "november",
	"december": "december", "dec": "december",
}

// ExtractMonths extracts canonical month names from text.
func ExtractMonths(text string) map[string]bool {
	set := make(map[string]bool)
	words := wsRe.Split(strings.ToLower(text), -1)
	for _, w := range words {
		w = nonAlphaRe.ReplaceAllString(w, "")
		if canonical, ok := monthNames[w]; ok {
			set[canonical] = true
		}
	}
	return set
}

// ExtractResolutionDates extracts resolution deadline dates from a description.
// Returns a set of "month day year" strings (e.g. "january 31 2026") found
// after keywords like "by", "before", "on".
func ExtractResolutionDates(text string) map[string]bool {
	set := make(map[string]bool)
	matches := resolutionDate.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		// m[1]=month, m[2]=day, m[3]=year
		date := strings.ToLower(m[1]) + " " + m[2] + " " + m[3]
		set[date] = true
	}
	return set
}

// subjectStopwords are common words that should NOT be treated as the subject.
var subjectStopwords = map[string]bool{
	"will": true, "does": true, "did": true, "has": true, "have": true,
	"can": true, "could": true, "would": true, "should": true,
	"is": true, "are": true, "was": true, "were": true, "be": true,
}

// ExtractSubject extracts the subject (first capitalized word/phrase after
// leading verb) from a question. For prediction market questions like
// "Will Opinion launch a token..." the subject is "Opinion".
// Returns lowercase subject tokens for comparison.
func ExtractSubject(text string) map[string]bool {
	set := make(map[string]bool)
	text = strings.TrimSpace(text)
	if text == "" {
		return set
	}

	// Remove trailing punctuation
	text = strings.TrimRight(text, "?!.")

	words := wsRe.Split(text, -1)

	// Skip leading stopwords/verbs to find the subject
	start := 0
	for start < len(words) {
		if subjectStopwords[strings.ToLower(words[start])] {
			start++
		} else {
			break
		}
	}
	if start >= len(words) {
		return set
	}

	// Collect consecutive capitalized words as the subject phrase.
	// Also include parenthetical tickers like "(TSLA)".
	for i := start; i < len(words); i++ {
		w := words[i]
		// Parenthetical like "(TSLA)" — include as subject
		if len(w) >= 3 && w[0] == '(' && w[len(w)-1] == ')' {
			inner := strings.ToLower(w[1 : len(w)-1])
			if inner != "" {
				set[inner] = true
			}
			continue
		}
		// Stop at first lowercase/stopword/verb
		if len(w) == 0 || (w[0] >= 'a' && w[0] <= 'z') {
			break
		}
		// Only include words that start with uppercase
		if w[0] >= 'A' && w[0] <= 'Z' {
			cleaned := strings.ToLower(regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAllString(w, ""))
			if cleaned != "" && !subjectStopwords[cleaned] {
				set[cleaned] = true
			}
		} else {
			break
		}
	}

	return set
}

// NormalizeOutcomeName normalizes an outcome name for comparison.
func NormalizeOutcomeName(name string) string {
	cleaned := strings.ToLower(name)
	cleaned = regexp.MustCompile(`[^a-z0-9]`).ReplaceAllString(cleaned, "")
	return strings.TrimSpace(cleaned)
}

var yesNames = map[string]bool{"yes": true, "true": true, "y": true, "1": true}
var noNames = map[string]bool{"no": true, "false": true, "n": true, "0": true}

// YesNoOutcomes holds identified yes/no outcomes.
type YesNoOutcomes struct {
	Yes int // index into outcomes
	No  int
}

// ExtractYesNoIndices finds yes/no outcome indices from a list of outcome names.
func ExtractYesNoIndices(names []string) (*YesNoOutcomes, bool) {
	yesIdx := -1
	noIdx := -1
	for i, name := range names {
		norm := NormalizeOutcomeName(name)
		if yesNames[norm] && yesIdx < 0 {
			yesIdx = i
		}
		if noNames[norm] && noIdx < 0 {
			noIdx = i
		}
	}
	if yesIdx < 0 || noIdx < 0 {
		return nil, false
	}
	return &YesNoOutcomes{Yes: yesIdx, No: noIdx}, true
}
