package matcher

import (
	"testing"

	"predict-market/internal/market"
)

func makeMarket(id, question string, outcomes []string, source string) market.NormalizedMarket {
	outs := make([]market.MarketOutcome, len(outcomes))
	for i, name := range outcomes {
		outs[i] = market.MarketOutcome{
			ID:    id + "_" + name,
			Name:  name,
			Index: i,
		}
	}
	return market.NormalizedMarket{
		ID:                   id,
		Question:             question,
		Outcomes:             outs,
		Source:               source,
		Category:             market.MarketCategory{Tags: []market.Tag{}},
		BulletinBoardUpdates: []market.BulletinBoardUpdate{},
	}
}

func defaultCfg() MatchConfig {
	return MatchConfig{
		MinSimilarity:               0.8,
		MinCharSimilarity:           0.78,
		MinMargin:                   0.08,
		MinTokens:                   4,
		RequireNumberMatch:          true,
		RequireYearMatch:            true,
		RequireMonthMatch:           true,
		RequireSubjectMatch:         true,
		RequireDescriptionDateMatch: true,
	}
}

func TestBuildPairsExactMatch(t *testing.T) {
	predict := []market.NormalizedMarket{
		makeMarket("p1", "Will Bitcoin reach $100,000 by December 2025?", []string{"Yes", "No"}, "Predict.Fun"),
	}
	poly := []market.NormalizedMarket{
		makeMarket("poly1", "Will Bitcoin reach $100,000 by December 2025?", []string{"Yes", "No"}, "Polymarket"),
	}

	pairs := BuildPairs(predict, poly, defaultCfg())
	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(pairs))
	}
	if pairs[0].Similarity < 0.99 {
		t.Errorf("similarity = %f, want >= 0.99", pairs[0].Similarity)
	}
}

func TestBuildPairsNoMatch(t *testing.T) {
	predict := []market.NormalizedMarket{
		makeMarket("p1", "Will Bitcoin reach $100,000 by December 2025?", []string{"Yes", "No"}, "Predict.Fun"),
	}
	poly := []market.NormalizedMarket{
		makeMarket("poly1", "Will Ethereum merge happen successfully?", []string{"Yes", "No"}, "Polymarket"),
	}

	pairs := BuildPairs(predict, poly, defaultCfg())
	if len(pairs) != 0 {
		t.Errorf("expected 0 pairs, got %d", len(pairs))
	}
	if pairs == nil {
		t.Error("expected empty slice, got nil")
	}
}

func TestBuildPairsYearMismatch(t *testing.T) {
	predict := []market.NormalizedMarket{
		makeMarket("p1", "Will Bitcoin reach $100,000 by December 2025?", []string{"Yes", "No"}, "Predict.Fun"),
	}
	poly := []market.NormalizedMarket{
		makeMarket("poly1", "Will Bitcoin reach $100,000 by December 2026?", []string{"Yes", "No"}, "Polymarket"),
	}

	pairs := BuildPairs(predict, poly, defaultCfg())
	if len(pairs) != 0 {
		t.Errorf("expected 0 pairs (year mismatch), got %d", len(pairs))
	}
}

func TestBuildPairsNoYesNo(t *testing.T) {
	predict := []market.NormalizedMarket{
		makeMarket("p1", "Who will win the 2025 presidential election?", []string{"Biden", "Trump"}, "Predict.Fun"),
	}
	poly := []market.NormalizedMarket{
		makeMarket("poly1", "Who will win the 2025 presidential election?", []string{"Biden", "Trump"}, "Polymarket"),
	}

	// Neither has Yes/No outcomes, so no pairs
	pairs := BuildPairs(predict, poly, defaultCfg())
	if len(pairs) != 0 {
		t.Errorf("expected 0 pairs (no yes/no), got %d", len(pairs))
	}
}

func TestBuildPairsSubjectMismatch(t *testing.T) {
	predict := []market.NormalizedMarket{
		makeMarket("p1", "Will Opinion launch a token by January 31, 2026?", []string{"Yes", "No"}, "Predict.Fun"),
	}
	poly := []market.NormalizedMarket{
		makeMarket("poly1", "Will Infinex launch a token by January 31, 2026?", []string{"Yes", "No"}, "Polymarket"),
	}

	pairs := BuildPairs(predict, poly, defaultCfg())
	if len(pairs) != 0 {
		t.Errorf("expected 0 pairs (subject mismatch: Opinion vs Infinex), got %d", len(pairs))
	}
}

func TestBuildPairsMonthMismatch(t *testing.T) {
	predict := []market.NormalizedMarket{
		makeMarket("p1", "Will Tesla (TSLA) close above $430 end of February?", []string{"Yes", "No"}, "Predict.Fun"),
	}
	poly := []market.NormalizedMarket{
		makeMarket("poly1", "Will Tesla (TSLA) close above $430 end of January?", []string{"Yes", "No"}, "Polymarket"),
	}

	pairs := BuildPairs(predict, poly, defaultCfg())
	if len(pairs) != 0 {
		t.Errorf("expected 0 pairs (month mismatch: February vs January), got %d", len(pairs))
	}
}

func TestBuildPairsDescriptionDateMismatch(t *testing.T) {
	predictDesc := "This market will resolve by January 31, 2026, 11:59 PM ET."
	polyDesc := "This market will resolve by December 31, 2026, 11:59 PM ET."

	predict := []market.NormalizedMarket{
		{
			ID:                   "p1",
			Question:             "Will Trump nominate Kevin Warsh as the next Fed chair?",
			Outcomes:             []market.MarketOutcome{{ID: "p1_Yes", Name: "Yes", Index: 0}, {ID: "p1_No", Name: "No", Index: 1}},
			Source:               "Predict.Fun",
			Category:             market.MarketCategory{Description: &predictDesc, Tags: []market.Tag{}},
			BulletinBoardUpdates: []market.BulletinBoardUpdate{},
		},
	}
	poly := []market.NormalizedMarket{
		{
			ID:                   "poly1",
			Question:             "Will Trump nominate Kevin Warsh as the next Fed chair?",
			Outcomes:             []market.MarketOutcome{{ID: "poly1_Yes", Name: "Yes", Index: 0}, {ID: "poly1_No", Name: "No", Index: 1}},
			Source:               "Polymarket",
			Category:             market.MarketCategory{Description: &polyDesc, Tags: []market.Tag{}},
			BulletinBoardUpdates: []market.BulletinBoardUpdate{},
		},
	}

	pairs := BuildPairs(predict, poly, defaultCfg())
	if len(pairs) != 0 {
		t.Errorf("expected 0 pairs (description date mismatch: Jan 31 vs Dec 31), got %d", len(pairs))
	}
}

func TestBuildPairsMonthOneSided(t *testing.T) {
	predict := []market.NormalizedMarket{
		makeMarket("p1", "Will Trump nominate Kevin Warsh as the next Fed chair by February 2026?", []string{"Yes", "No"}, "Predict.Fun"),
	}
	poly := []market.NormalizedMarket{
		makeMarket("poly1", "Will Trump nominate Kevin Warsh as the next Fed chair?", []string{"Yes", "No"}, "Polymarket"),
	}

	pairs := BuildPairs(predict, poly, defaultCfg())
	if len(pairs) != 0 {
		t.Errorf("expected 0 pairs (one side has month, other doesn't), got %d", len(pairs))
	}
}

func TestBuildPairsSubjectMatchCorrect(t *testing.T) {
	predict := []market.NormalizedMarket{
		makeMarket("p1", "Will Opinion launch a token by March 31, 2026?", []string{"Yes", "No"}, "Predict.Fun"),
	}
	poly := []market.NormalizedMarket{
		makeMarket("poly1", "Will Opinion launch a token by March 31, 2026?", []string{"Yes", "No"}, "Polymarket"),
	}

	pairs := BuildPairs(predict, poly, defaultCfg())
	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair (same subject), got %d", len(pairs))
	}
}

func TestBuildPairsMarginFilter(t *testing.T) {
	predict := []market.NormalizedMarket{
		makeMarket("p1", "Will artificial intelligence replace most jobs by 2030?", []string{"Yes", "No"}, "Predict.Fun"),
	}
	poly := []market.NormalizedMarket{
		makeMarket("poly1", "Will artificial intelligence replace most jobs by 2030?", []string{"Yes", "No"}, "Polymarket"),
		makeMarket("poly2", "Will artificial intelligence replace most human jobs by 2030?", []string{"Yes", "No"}, "Polymarket"),
	}

	cfg := MatchConfig{
		MinSimilarity:               0.5,
		MinCharSimilarity:           0.5,
		MinMargin:                   0.5, // Very high margin
		MinTokens:                   3,
		RequireNumberMatch:          false,
		RequireYearMatch:            false,
		RequireMonthMatch:           false,
		RequireSubjectMatch:         false,
		RequireDescriptionDateMatch: false,
	}

	// Both poly markets are very similar to predict, so margin filter should reject
	pairs := BuildPairs(predict, poly, cfg)
	if len(pairs) != 0 {
		t.Errorf("expected 0 pairs (margin filter), got %d", len(pairs))
	}
}
