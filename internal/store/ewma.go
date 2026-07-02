package store

import "time"

// DefaultAlpha is the smoothing factor for EWMA (exponentially-weighted
// moving average). Higher values give more weight to recent rounds.
const DefaultAlpha = 0.3

// UpdateMastery computes a new Mastery value from a round result and the
// previous Mastery state.
//
// The round score is a composite of:
//   - efficiency:  par / keystrokes (capped at 1.0)
//   - star rating: stars / 3.0
//
// The two components are averaged to produce a score in [0, 1].
// EWMA: new_value = α * score + (1-α) * old_value
// For the first round (no prior data) the score becomes the value directly.
func UpdateMastery(prev Mastery, keystrokes, par, stars int, alpha float64) Mastery {
	if par <= 0 {
		// No par available — use star rating alone.
		par = keystrokes
	}

	// efficiency: how close to optimal
	efficiency := float64(par) / float64(keystrokes)
	if efficiency > 1.0 {
		efficiency = 1.0
	}

	// star score
	starScore := float64(stars) / 3.0

	// composite score
	score := (efficiency + starScore) / 2.0

	var newValue float64
	if prev.Rounds == 0 {
		newValue = score
	} else {
		newValue = alpha*score + (1.0-alpha)*prev.Value
	}

	return Mastery{
		Value:      newValue,
		Rounds:     prev.Rounds + 1,
		LastPlayed: time.Now(),
	}
}

// UpdateBestScore returns the best (lowest keystrokes, highest stars)
// between the current best and a new result.
func UpdateBestScore(current BestScore, keystrokes, par, stars int) BestScore {
	if current.Stars == 0 && current.Keystrokes == 0 {
		// No prior best — this is the first.
		return BestScore{
			Keystrokes: keystrokes,
			Par:        par,
			Stars:      stars,
		}
	}

	// Compare: more stars is better; tie goes to fewer keystrokes.
	if stars > current.Stars || (stars == current.Stars && keystrokes < current.Keystrokes) {
		return BestScore{
			Keystrokes: keystrokes,
			Par:        par,
			Stars:      stars,
		}
	}

	return current
}