package curriculum_test

import (
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