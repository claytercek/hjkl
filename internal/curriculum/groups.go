// Package curriculum defines the progression model, including Motion Groups
// as the unit of progression.
package curriculum

import (
	"fmt"

	"github.com/clay/hjkl/internal/challenge"
)

// MotionGroup is the unit of progression. Each group bundles related motion
// keys that unlock together.
type MotionGroup struct {
	// Key is a short identifier used in persistence (e.g. "hjkl").
	Key string

	// Name is a short human-readable name (e.g. "Basic Movement").
	Name string

	// Pitch is a one-line description of why the group is powerful.
	Pitch string

	// Keys lists the motion keystrokes in this group.
	Keys []string
}

// Groups defines the authored unlock order. The first group is the starting
// vocabulary; each subsequent group unlocks when the previous reaches
// sufficient mastery.
var Groups = []MotionGroup{
	{
		Key:   "hjkl",
		Name:  "Basic Movement",
		Pitch: "Move the cursor in all four cardinal directions",
		Keys:  []string{"h", "j", "k", "l"},
	},
	{
		Key:   "wbe",
		Name:  "Word Navigation",
		Pitch: "Hop between words with precision",
		Keys:  []string{"w", "b", "e"},
	},
	{
		Key:   "0^$",
		Name:  "Line Edges",
		Pitch: "Jump to the start or end of a line instantly",
		Keys:  []string{"0", "^", "$"},
	},
	{
		Key:   "ft;",
		Name:  "Find Character",
		Pitch: "Land on any character in a single bound",
		Keys:  []string{"f", "t", "F", "T", ";"},
	},
	{
		Key:   "ggG",
		Name:  "File Navigation",
		Pitch: "Scout the entire file in two keystrokes",
		Keys:  []string{"g", "G"},
	},
	{
		Key:   "WBE",
		Name:  "WORD Navigation",
		Pitch: "Skip entire chunks of text at once",
		Keys:  []string{"W", "B", "E"},
	},
}

// StartGroup returns the first (starting) motion group.
func StartGroup() MotionGroup {
	return Groups[0]
}

// StartingVocabulary returns the keys in the first group.
func StartingVocabulary() []string {
	return Groups[0].Keys
}

// GroupForTemplate returns the Motion Group key that a challenge template
// primarily exercises. This is the single source of truth linking generated
// challenges to progression groups.
func GroupForTemplate(tmpl challenge.TemplateKind) string {
	switch tmpl {
	case challenge.THorizontalLine:
		return "hjkl"
	case challenge.TVerticalNavigation:
		return "hjkl"
	case challenge.TFindCharacter:
		return "ft;"
	default:
		panic(fmt.Sprintf("curriculum: unmapped template %v", tmpl))
	}
}

// MasteryThreshold is the mastery value at which a Motion Group is considered
// unlocked, making the next group the new frontier.
const MasteryThreshold = 0.7

// FrontierProgress returns the index of the frontier group (the first
// group whose mastery is below the threshold), and the ratio of its
// current mastery to the threshold as a value in [0.0, 1.0].
//
// If all groups are unlocked (mastery >= threshold), frontierIdx is -1
// and ratio is 1.0.
func FrontierProgress(mastery map[string]float64) (frontierIdx int, ratio float64) {
	// Groups[0] is the starting vocabulary, always unlocked.
	// Start checking from index 1.
	for i := 1; i < len(Groups); i++ {
		val := mastery[Groups[i].Key]
		if val < MasteryThreshold {
			// This is the frontier group.
			p := val / MasteryThreshold
			if p > 1.0 {
				p = 1.0
			}
			return i, p
		}
	}
	return -1, 1.0
}

// GroupForMotion returns the MotionGroup that the given motion key belongs to,
// or nil if the motion is not part of any authored group (e.g. target
// characters for f/t).
func GroupForMotion(key string) *MotionGroup {
	for i := range Groups {
		for _, k := range Groups[i].Keys {
			if k == key {
				return &Groups[i]
			}
		}
	}
	return nil
}
