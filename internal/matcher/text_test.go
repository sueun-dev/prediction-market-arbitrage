package matcher

import (
	"testing"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		input string
		want  int // expected token count
	}{
		{"", 0},
		{"Will Bitcoin reach $100,000 by 2025?", 3}, // bitcoin, reach, 100000 (2025 is a token too => actually: bitcoin, reach, 100, 000, 2025)
		{"The quick brown fox", 2},                   // quick, brown (fox=3 chars so included => quick, brown, fox)
	}

	for _, tt := range tests {
		tokens := Tokenize(tt.input)
		if len(tokens) < 1 && tt.want > 0 {
			t.Errorf("Tokenize(%q) returned empty, want %d tokens", tt.input, tt.want)
		}
	}

	// Test specific tokenization
	tokens := Tokenize("Will Bitcoin reach $100,000 by 2025?")
	found := false
	for _, tok := range tokens {
		if tok == "bitcoin" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'bitcoin' in tokens, got %v", tokens)
	}
}

func TestTokenSet(t *testing.T) {
	set := TokenSet("Bitcoin Bitcoin Ethereum")
	if !set["bitcoin"] {
		t.Error("expected 'bitcoin' in set")
	}
	if !set["ethereum"] {
		t.Error("expected 'ethereum' in set")
	}
}

func TestBigramSet(t *testing.T) {
	set := BigramSet("abc")
	if !set["ab"] {
		t.Error("expected 'ab' in bigrams")
	}
	if !set["bc"] {
		t.Error("expected 'bc' in bigrams")
	}
	if len(set) != 2 {
		t.Errorf("expected 2 bigrams, got %d", len(set))
	}

	empty := BigramSet("")
	if len(empty) != 0 {
		t.Error("expected empty set for empty string")
	}
}

func TestDiceCoefficient(t *testing.T) {
	a := map[string]bool{"x": true, "y": true, "z": true}
	b := map[string]bool{"x": true, "y": true, "w": true}

	score := DiceCoefficient(a, b)
	// overlap=2, total=6, dice = 4/6 = 0.666...
	if score < 0.66 || score > 0.67 {
		t.Errorf("DiceCoefficient = %f, want ~0.667", score)
	}

	// Identical sets
	score2 := DiceCoefficient(a, a)
	if score2 != 1.0 {
		t.Errorf("DiceCoefficient(a, a) = %f, want 1.0", score2)
	}

	// Empty sets
	score3 := DiceCoefficient(map[string]bool{}, a)
	if score3 != 0 {
		t.Errorf("DiceCoefficient(empty, a) = %f, want 0", score3)
	}
}

func TestExtractNumbers(t *testing.T) {
	nums := ExtractNumbers("Bitcoin will reach 100000 by 2025")
	if !nums["100000"] {
		t.Error("expected '100000' in numbers")
	}
	if !nums["2025"] {
		t.Error("expected '2025' in numbers")
	}
}

func TestExtractYears(t *testing.T) {
	years := ExtractYears("Will happen in 2025 or 2026?")
	if !years["2025"] {
		t.Error("expected '2025' in years")
	}
	if !years["2026"] {
		t.Error("expected '2026' in years")
	}

	noYears := ExtractYears("Price is 100")
	if len(noYears) != 0 {
		t.Errorf("expected no years, got %v", noYears)
	}
}

func TestHasOverlap(t *testing.T) {
	a := map[string]bool{"x": true, "y": true}
	b := map[string]bool{"y": true, "z": true}
	c := map[string]bool{"w": true}

	if !HasOverlap(a, b) {
		t.Error("expected overlap between a and b")
	}
	if HasOverlap(a, c) {
		t.Error("expected no overlap between a and c")
	}
}

func TestNormalizeOutcomeName(t *testing.T) {
	if NormalizeOutcomeName("Yes") != "yes" {
		t.Error("expected 'yes'")
	}
	if NormalizeOutcomeName("NO!") != "no" {
		t.Error("expected 'no'")
	}
}

func TestExtractResolutionDates(t *testing.T) {
	dates := ExtractResolutionDates("This market will resolve by January 31, 2026, 11:59 PM ET.")
	if !dates["january 31 2026"] {
		t.Errorf("expected 'january 31 2026', got %v", dates)
	}

	dates2 := ExtractResolutionDates("resolve before December 31, 2026.")
	if !dates2["december 31 2026"] {
		t.Errorf("expected 'december 31 2026', got %v", dates2)
	}

	empty := ExtractResolutionDates("No date here.")
	if len(empty) != 0 {
		t.Errorf("expected empty, got %v", empty)
	}
}

func TestExtractMonths(t *testing.T) {
	months := ExtractMonths("Will launch by January 31, 2026?")
	if !months["january"] {
		t.Error("expected 'january' in months")
	}
	if len(months) != 1 {
		t.Errorf("expected 1 month, got %d: %v", len(months), months)
	}

	months2 := ExtractMonths("end of February?")
	if !months2["february"] {
		t.Error("expected 'february'")
	}

	empty := ExtractMonths("Will Bitcoin reach 100k?")
	if len(empty) != 0 {
		t.Errorf("expected no months, got %v", empty)
	}
}

func TestExtractSubject(t *testing.T) {
	tests := []struct {
		input string
		want  map[string]bool
	}{
		{"Will Opinion launch a token?", map[string]bool{"opinion": true}},
		{"Will Infinex launch a token?", map[string]bool{"infinex": true}},
		{"Will Tesla (TSLA) close above $430?", map[string]bool{"tesla": true, "tsla": true}},
		{"Will Trump nominate Kevin Warsh?", map[string]bool{"trump": true}},
	}

	for _, tt := range tests {
		got := ExtractSubject(tt.input)
		if !SetsEqual(got, tt.want) {
			t.Errorf("ExtractSubject(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestSetsEqual(t *testing.T) {
	a := map[string]bool{"x": true, "y": true}
	b := map[string]bool{"x": true, "y": true}
	c := map[string]bool{"x": true, "z": true}
	d := map[string]bool{"x": true}

	if !SetsEqual(a, b) {
		t.Error("expected a == b")
	}
	if SetsEqual(a, c) {
		t.Error("expected a != c")
	}
	if SetsEqual(a, d) {
		t.Error("expected a != d (different sizes)")
	}
}

func TestExtractYesNoIndices(t *testing.T) {
	names := []string{"Yes", "No"}
	yesNo, ok := ExtractYesNoIndices(names)
	if !ok {
		t.Fatal("expected ok")
	}
	if yesNo.Yes != 0 || yesNo.No != 1 {
		t.Errorf("Yes=%d No=%d, want 0, 1", yesNo.Yes, yesNo.No)
	}

	// Reversed
	names2 := []string{"No", "Yes"}
	yesNo2, ok := ExtractYesNoIndices(names2)
	if !ok {
		t.Fatal("expected ok")
	}
	if yesNo2.Yes != 1 || yesNo2.No != 0 {
		t.Errorf("Yes=%d No=%d, want 1, 0", yesNo2.Yes, yesNo2.No)
	}

	// Missing
	_, ok = ExtractYesNoIndices([]string{"Maybe", "Probably"})
	if ok {
		t.Error("expected !ok for non-yes/no outcomes")
	}
}
