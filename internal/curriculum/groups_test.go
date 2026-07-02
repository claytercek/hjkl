package curriculum_test

import (
	"math"
	"testing"

	"github.com/clay/hjkl/internal/challenge"
	"github.com/clay/hjkl/internal/curriculum"
)

func TestGroups_UnlockOrder(t *testing.T) {
	groups := curriculum.Groups
	if len(groups) == 0 {
		t.Fatal("expected at least one motion group")
	}

	// First group should be the starting vocabulary.
	start := groups[0]
	if start.Key != "hjkl" {
		t.Fatalf("first group key = %q, want %q", start.Key, "hjkl")
	}
	if len(start.Keys) == 0 {
		t.Fatal("first group should have keys")
	}
}

func TestStartingVocabulary(t *testing.T) {
	vocab := curriculum.StartingVocabulary()
	if len(vocab) == 0 {
		t.Fatal("starting vocabulary should not be empty")
	}

	// Should contain h, j, k, l.
	expected := map[string]bool{"h": true, "j": true, "k": true, "l": true}
	for _, k := range vocab {
		if !expected[k] {
			t.Errorf("unexpected key %q in starting vocabulary", k)
		}
		delete(expected, k)
	}
	if len(expected) > 0 {
		t.Errorf("missing keys from starting vocabulary: %v", expected)
	}
}

func TestStartGroup(t *testing.T) {
	g := curriculum.StartGroup()
	if g.Key != "hjkl" {
		t.Fatalf("StartGroup().Key = %q, want %q", g.Key, "hjkl")
	}
	if g.Name == "" {
		t.Fatal("StartGroup().Name should not be empty")
	}
	if g.Pitch == "" {
		t.Fatal("StartGroup().Pitch should not be empty")
	}
}

func TestGroupForMotion_Found(t *testing.T) {
	g := curriculum.GroupForMotion("h")
	if g == nil {
		t.Fatal("GroupForMotion(\"h\") returned nil")
	}
	if g.Key != "hjkl" {
		t.Fatalf("GroupForMotion(\"h\") group = %q, want %q", g.Key, "hjkl")
	}

	g = curriculum.GroupForMotion("G")
	if g == nil {
		t.Fatal("GroupForMotion(\"G\") returned nil")
	}
	if g.Key != "ggG" {
		t.Fatalf("GroupForMotion(\"G\") group = %q, want %q", g.Key, "ggG")
	}

	g = curriculum.GroupForMotion("f")
	if g == nil {
		t.Fatal("GroupForMotion(\"f\") returned nil")
	}
	if g.Key != "ft;" {
		t.Fatalf("GroupForMotion(\"f\") group = %q, want %q", g.Key, "ft;")
	}
}

func TestGroupForMotion_NotFound(t *testing.T) {
	g := curriculum.GroupForMotion("z")
	if g != nil {
		t.Fatalf("GroupForMotion(\"z\") = %+v, want nil", g)
	}

	g = curriculum.GroupForMotion("")
	if g != nil {
		t.Fatalf("GroupForMotion(\"\") = %+v, want nil", g)
	}
}

func TestAllGroupsHaveNameAndPitch(t *testing.T) {
	for _, g := range curriculum.Groups {
		if g.Name == "" {
			t.Errorf("group %q has empty Name", g.Key)
		}
		if g.Pitch == "" {
			t.Errorf("group %q has empty Pitch", g.Key)
		}
	}
}

func TestAllGroupsHaveKeys(t *testing.T) {
	for _, g := range curriculum.Groups {
		if len(g.Keys) == 0 {
			t.Errorf("group %q has no keys", g.Key)
		}
	}
}

func TestGroupForTemplate(t *testing.T) {
	tests := []struct {
		tmpl challenge.TemplateKind
		want string
	}{
		{challenge.THorizontalLine, "hjkl"},
		{challenge.TVerticalNavigation, "hjkl"},
		{challenge.TFindCharacter, "ft;"},
	}

	for _, tt := range tests {
		t.Run(tt.tmpl.String(), func(t *testing.T) {
			got := curriculum.GroupForTemplate(tt.tmpl)
			if got != tt.want {
				t.Errorf("GroupForTemplate(%s) = %q, want %q", tt.tmpl, got, tt.want)
			}
		})
	}
}

func TestGroupForTemplate_PanicsOnUnknown(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for unknown template")
		}
	}()
	curriculum.GroupForTemplate(challenge.TemplateKind(999))
}

func TestFrontierProgress_NoMastery(t *testing.T) {
	// No mastery data yet — first unlockable group (wbe) is the frontier.
	mastery := map[string]float64{}
	idx, ratio := curriculum.FrontierProgress(mastery)
	if idx != 1 {
		t.Fatalf("frontierIdx = %d, want 1", idx)
	}
	if ratio != 0.0 {
		t.Fatalf("ratio = %f, want 0.0", ratio)
	}
}

func TestFrontierProgress_PartialMastery(t *testing.T) {
	// wbe at 0.35 / 0.7 = 0.5 ratio
	mastery := map[string]float64{"wbe": 0.35}
	idx, ratio := curriculum.FrontierProgress(mastery)
	if idx != 1 {
		t.Fatalf("frontierIdx = %d, want 1", idx)
	}
	if math.Abs(ratio-0.5) > 1e-9 {
		t.Fatalf("ratio = %f, want 0.5", ratio)
	}
}

func TestFrontierProgress_FirstGroupUnlocked(t *testing.T) {
	// wbe is at threshold, so 0^$ is the frontier (with 0 mastery).
	mastery := map[string]float64{"wbe": 0.7}
	idx, ratio := curriculum.FrontierProgress(mastery)
	if idx != 2 {
		t.Fatalf("frontierIdx = %d, want 2", idx)
	}
	if ratio != 0.0 {
		t.Fatalf("ratio = %f, want 0.0", ratio)
	}
}

func TestFrontierProgress_MultipleUnlocked(t *testing.T) {
	mastery := map[string]float64{"wbe": 0.8, "0^$": 0.75, "ft;": 0.3}
	idx, ratio := curriculum.FrontierProgress(mastery)
	if idx != 3 {
		t.Fatalf("frontierIdx = %d, want 3", idx)
	}
	expected := 0.3 / 0.7
	if math.Abs(ratio-expected) > 1e-9 {
		t.Fatalf("ratio = %f, want %f", ratio, expected)
	}
}

func TestFrontierProgress_AllUnlocked(t *testing.T) {
	mastery := map[string]float64{
		"wbe": 0.8,
		"0^$": 0.75,
		"ft;": 0.9,
		"ggG": 0.85,
		"WBE": 0.95,
	}
	idx, ratio := curriculum.FrontierProgress(mastery)
	if idx != -1 {
		t.Fatalf("frontierIdx = %d, want -1", idx)
	}
	if ratio != 1.0 {
		t.Fatalf("ratio = %f, want 1.0", ratio)
	}
}

func TestFrontierProgress_ExactThreshold(t *testing.T) {
	// Exactly at threshold counts as unlocked.
	mastery := map[string]float64{"wbe": 0.7}
	idx, ratio := curriculum.FrontierProgress(mastery)
	if idx != 2 {
		t.Fatalf("frontierIdx = %d, want 2 (skip wbe at threshold)", idx)
	}
	if ratio != 0.0 {
		t.Fatalf("ratio = %f, want 0.0", ratio)
	}
}

func TestFrontierProgress_AboveThreshold(t *testing.T) {
	mastery := map[string]float64{"wbe": 0.9}
	idx, ratio := curriculum.FrontierProgress(mastery)
	if idx != 2 {
		t.Fatalf("frontierIdx = %d, want 2", idx)
	}
	if ratio != 0.0 {
		t.Fatalf("ratio = %f, want 0.0", ratio)
	}
}

func TestFrontierProgress_NearThreshold(t *testing.T) {
	mastery := map[string]float64{"wbe": 0.65}
	idx, ratio := curriculum.FrontierProgress(mastery)
	if idx != 1 {
		t.Fatalf("frontierIdx = %d, want 1", idx)
	}
	expected := 0.65 / 0.7
	if math.Abs(ratio-expected) > 1e-9 {
		t.Fatalf("ratio = %f, want %f", ratio, expected)
	}
}

func TestUnlockedVocabulary_AllUnlocked(t *testing.T) {
	mastery := map[string]float64{
		"wbe": 0.8,
		"0^$": 0.75,
		"ft;": 0.9,
		"ggG": 0.85,
		"WBE": 0.95,
	}
	vocab := curriculum.UnlockedVocabulary(mastery)
	if len(vocab) == 0 {
		t.Fatal("expected non-empty vocabulary")
	}
	// Should include all group keys.
	allKeys := map[string]bool{}
	for _, g := range curriculum.Groups {
		for _, k := range g.Keys {
			allKeys[k] = true
		}
	}
	for _, k := range vocab {
		delete(allKeys, k)
	}
	if len(allKeys) > 0 {
		t.Errorf("missing keys: %v", allKeys)
	}
}

func TestUnlockedVocabulary_NothingUnlocked(t *testing.T) {
	// Starting group is always unlocked.
	vocab := curriculum.UnlockedVocabulary(map[string]float64{})
	if len(vocab) != 4 {
		t.Fatalf("expected 4 keys (hjkl), got %d: %v", len(vocab), vocab)
	}
	for _, k := range []string{"h", "j", "k", "l"} {
		found := false
		for _, v := range vocab {
			if v == k {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing key %q", k)
		}
	}
}

func TestUnlockedVocabulary_OnlyFirstUnlocked(t *testing.T) {
	mastery := map[string]float64{"wbe": 0.8}
	vocab := curriculum.UnlockedVocabulary(mastery)
	// Should include hjkl + wbe keys.
	expected := []string{"h", "j", "k", "l", "w", "b", "e"}
	if len(vocab) != len(expected) {
		t.Fatalf("expected %d keys, got %d: %v", len(expected), len(vocab), vocab)
	}
}

func TestTemplatesForGroup_Hjkl(t *testing.T) {
	templates := curriculum.TemplatesForGroup("hjkl")
	if len(templates) != 2 {
		t.Fatalf("expected 2 templates for hjkl, got %d", len(templates))
	}
}

func TestTemplatesForGroup_FtSemi(t *testing.T) {
	templates := curriculum.TemplatesForGroup("ft;")
	if len(templates) != 1 {
		t.Fatalf("expected 1 template for ft;, got %d", len(templates))
	}
}

func TestTemplatesForGroup_Unknown(t *testing.T) {
	templates := curriculum.TemplatesForGroup("wbe")
	// Unknown groups fall back to all templates.
	if len(templates) == 0 {
		t.Fatal("expected fallback templates for unknown group")
	}
}
