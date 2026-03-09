package matcher

import (
	"fmt"
	"math"

	"predict-market/internal/market"
)

// MatchConfig holds pair matching configuration.
type MatchConfig struct {
	MinSimilarity               float64
	MinCharSimilarity           float64
	MinMargin                   float64
	MinTokens                   int
	RequireNumberMatch          bool
	RequireYearMatch            bool
	RequireMonthMatch           bool
	RequireSubjectMatch         bool
	RequireDescriptionDateMatch bool
}

type polyIndexEntry struct {
	market    market.NormalizedMarket
	tokens    map[string]bool
	bigrams   map[string]bool
	numbers   map[string]bool
	years     map[string]bool
	months    map[string]bool
	subject   map[string]bool
	descDates map[string]bool
	yesNo     YesNoOutcomes
}

type scoredCandidate struct {
	score float64
	entry *polyIndexEntry
}

// BuildPairs matches Predict.Fun markets with Polymarket markets.
func BuildPairs(
	predictMarkets []market.NormalizedMarket,
	polymarketMarkets []market.NormalizedMarket,
	cfg MatchConfig,
) []market.MarketPair {
	// Build Polymarket index
	var polyIndex []polyIndexEntry
	for _, m := range polymarketMarkets {
		names := outcomeNames(m.Outcomes)
		yesNo, ok := ExtractYesNoIndices(names)
		if !ok {
			continue
		}
		question := m.Question
		tokens := TokenSet(question)
		if len(tokens) < cfg.MinTokens {
			continue
		}
		var polyDesc string
		if m.Category.Description != nil {
			polyDesc = *m.Category.Description
		}
		polyIndex = append(polyIndex, polyIndexEntry{
			market:    m,
			tokens:    tokens,
			bigrams:   BigramSet(question),
			numbers:   ExtractNumbers(question),
			years:     ExtractYears(question),
			months:    ExtractMonths(question),
			subject:   ExtractSubject(question),
			descDates: ExtractResolutionDates(polyDesc),
			yesNo:     *yesNo,
		})
	}

	usedPoly := make(map[string]bool)
	pairs := make([]market.MarketPair, 0)

	for _, predict := range predictMarkets {
		names := outcomeNames(predict.Outcomes)
		predictYesNo, ok := ExtractYesNoIndices(names)
		if !ok {
			continue
		}
		question := predict.Question
		predictTokens := TokenSet(question)
		if len(predictTokens) < cfg.MinTokens {
			continue
		}
		predictBigrams := BigramSet(question)
		predictNumbers := ExtractNumbers(question)
		predictYears := ExtractYears(question)
		predictMonths := ExtractMonths(question)
		predictSubject := ExtractSubject(question)
		var predictDesc string
		if predict.Category.Description != nil {
			predictDesc = *predict.Category.Description
		}
		predictDescDates := ExtractResolutionDates(predictDesc)

		var best, second *scoredCandidate

		for i := range polyIndex {
			entry := &polyIndex[i]
			if usedPoly[entry.market.ID] {
				continue
			}

			if cfg.RequireYearMatch && len(predictYears) > 0 && len(entry.years) > 0 {
				if !HasOverlap(predictYears, entry.years) {
					continue
				}
			}
			if cfg.RequireNumberMatch && len(predictNumbers) > 0 && len(entry.numbers) > 0 {
				if !HasOverlap(predictNumbers, entry.numbers) {
					continue
				}
			}
			if cfg.RequireMonthMatch {
				pm := len(predictMonths) > 0
				em := len(entry.months) > 0
				if pm != em {
					// One side has a month and the other doesn't — likely different expiry
					continue
				}
				if pm && em && !SetsEqual(predictMonths, entry.months) {
					continue
				}
			}
			if cfg.RequireSubjectMatch && len(predictSubject) > 0 && len(entry.subject) > 0 {
				if !SetsEqual(predictSubject, entry.subject) {
					continue
				}
			}
			if cfg.RequireDescriptionDateMatch && len(predictDescDates) > 0 && len(entry.descDates) > 0 {
				if !SetsEqual(predictDescDates, entry.descDates) {
					continue
				}
			}

			tokenScore := DiceCoefficient(predictTokens, entry.tokens)
			if tokenScore < cfg.MinSimilarity {
				continue
			}
			charScore := DiceCoefficient(predictBigrams, entry.bigrams)
			if charScore < cfg.MinCharSimilarity {
				continue
			}

			score := (tokenScore + charScore) / 2
			if best == nil || score > best.score {
				second = best
				best = &scoredCandidate{score: score, entry: entry}
			} else if second == nil || score > second.score {
				second = &scoredCandidate{score: score, entry: entry}
			}
		}

		if best == nil || best.score < cfg.MinSimilarity {
			continue
		}
		if second != nil && best.score-second.score < cfg.MinMargin {
			continue
		}

		usedPoly[best.entry.market.ID] = true

		pairs = append(pairs, market.MarketPair{
			ID:         fmt.Sprintf("pair_%s_%s", predict.ID, best.entry.market.ID),
			Similarity: math.Round(best.score*10000) / 10000,
			Question:   predict.Question,
			Predict:    trimPredict(predict),
			Polymarket: trimPolymarket(best.entry.market),
			Pricing: market.PairPricing{
				Predict: market.YesNoPricing{
					Yes: buildPricing(predict.Outcomes, predictYesNo.Yes),
					No:  buildPricing(predict.Outcomes, predictYesNo.No),
				},
				Polymarket: market.YesNoPricing{
					Yes: buildPricing(best.entry.market.Outcomes, best.entry.yesNo.Yes),
					No:  buildPricing(best.entry.market.Outcomes, best.entry.yesNo.No),
				},
			},
		})
	}

	return pairs
}

func outcomeNames(outcomes []market.MarketOutcome) []string {
	names := make([]string, len(outcomes))
	for i, o := range outcomes {
		names[i] = o.Name
	}
	return names
}

func trimPredict(m market.NormalizedMarket) market.TrimmedPredictMarket {
	return market.TrimmedPredictMarket{
		ID:             m.ID,
		Question:       m.Question,
		Category:       m.Category,
		Statistics:     m.Statistics,
		TotalPositions: m.TotalPositions,
		Orderbook:      m.Orderbook,
		Source:         m.Source,
		SourceUrl:      m.SourceUrl,
	}
}

func trimPolymarket(m market.NormalizedMarket) market.TrimmedPolymarketMarket {
	return market.TrimmedPolymarketMarket{
		ID:              m.ID,
		Question:        m.Question,
		Category:        m.Category,
		Statistics:      m.Statistics,
		Orderbook:       m.Orderbook,
		OrderbookTokens: m.OrderbookTokens,
		Source:          m.Source,
		SourceUrl:       m.SourceUrl,
	}
}

func buildPricing(outcomes []market.MarketOutcome, idx int) market.OutcomePricing {
	if idx < 0 || idx >= len(outcomes) {
		return market.OutcomePricing{}
	}
	o := outcomes[idx]
	bid := coalesceFloat(o.BidPriceInCurrency, o.BidPrice)
	ask := coalesceFloat(o.AskPriceInCurrency, o.AskPrice)
	return market.OutcomePricing{
		Bid:   bid,
		Ask:   ask,
		Price: o.Price,
	}
}

func coalesceFloat(vals ...*float64) *float64 {
	for _, v := range vals {
		if v != nil {
			return v
		}
	}
	return nil
}
