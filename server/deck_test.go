package main

import "testing"

func TestFaceIDOf(t *testing.T) {
	cases := map[int]string{
		1:  "1_club",
		2:  "2_club",
		10: "10_club",
		11: "jack_club",
		12: "queen_club",
		13: "king_club",
	}
	for rank, want := range cases {
		if got := faceIDOf("club", rank); got != want {
			t.Errorf("faceIDOf(club,%d) = %q, attendu %q", rank, got, want)
		}
	}
}

func TestBuildDeckFull52(t *testing.T) {
	cards, err := BuildDeck(DefaultDeckConfig())
	if err != nil {
		t.Fatalf("BuildDeck a échoué: %v", err)
	}
	if len(cards) != 52 {
		t.Fatalf("un jeu classique devrait avoir 52 cartes, en a %d", len(cards))
	}
	// Aucun ID en doublon.
	seen := map[string]bool{}
	for _, c := range cards {
		if seen[c.ID] {
			t.Fatalf("ID en doublon: %s", c.ID)
		}
		seen[c.ID] = true
	}
	// Aucun faceId "back" (le dos n'est jamais une carte).
	for _, c := range cards {
		if c.FaceID == "back" {
			t.Fatal("le dos ne doit pas figurer comme carte du sabot")
		}
	}
}

func TestBuildDeckPartialSuits(t *testing.T) {
	// Deux couleurs, rangs 2..13 (pas d'As) -> 2 * 12 = 24 cartes.
	cfg := DeckConfig{
		DeckCount: 1,
		Suits:     []string{"heart", "spade"},
		FromRank:  2,
		ToRank:    13,
		Jokers:    "none",
	}
	cards, err := BuildDeck(cfg)
	if err != nil {
		t.Fatalf("BuildDeck a échoué: %v", err)
	}
	if len(cards) != 24 {
		t.Fatalf("attendu 24 cartes, en a %d", len(cards))
	}
	for _, c := range cards {
		if c.FaceID == "1_heart" || c.FaceID == "1_spade" {
			t.Fatalf("l'As ne devrait pas être inclus: %s", c.FaceID)
		}
	}
}

func TestBuildDeckJokers(t *testing.T) {
	for _, tc := range []struct {
		jokers string
		want   int
	}{
		{"none", 0},
		{"black", 1},
		{"red", 1},
		{"both", 2},
	} {
		cfg := DefaultDeckConfig()
		cfg.Jokers = tc.jokers
		cards, err := BuildDeck(cfg)
		if err != nil {
			t.Fatalf("BuildDeck(%s) a échoué: %v", tc.jokers, err)
		}
		if len(cards) != 52+tc.want {
			t.Errorf("jokers=%s: attendu %d cartes, en a %d", tc.jokers, 52+tc.want, len(cards))
		}
	}
}

func TestBuildDeckMultipleDecks(t *testing.T) {
	cfg := DefaultDeckConfig()
	cfg.DeckCount = 2
	cards, err := BuildDeck(cfg)
	if err != nil {
		t.Fatalf("BuildDeck a échoué: %v", err)
	}
	if len(cards) != 104 {
		t.Fatalf("2 jeux devraient produire 104 cartes, en a %d", len(cards))
	}
}

func TestNormalizeIsPermissiveButSane(t *testing.T) {
	// Normalize est volontairement permissive : une config dégénérée retombe
	// sur des valeurs saines (défauts) plutôt que d'échouer. On vérifie ici que
	// les bornes inversées sont remises dans l'ordre et qu'aucune couleur
	// valide ne se perd.
	cfg := DeckConfig{DeckCount: 0, Suits: nil, FromRank: 5, ToRank: 2}
	n, err := cfg.Normalize()
	if err != nil {
		t.Fatalf("Normalize ne devrait pas échouer sur une config dégénérée: %v", err)
	}
	if n.DeckCount < 1 {
		t.Fatal("DeckCount devrait être clampé à >= 1")
	}
	if n.FromRank > n.ToRank {
		t.Fatal("les bornes inversées devraient être réordonnées")
	}
	if len(n.Suits) == 0 {
		t.Fatal("les couleurs devraient retomber sur les 4 par défaut")
	}
	// La config normalisée doit produire au moins une carte.
	cards, err := BuildDeck(n)
	if err != nil || len(cards) == 0 {
		t.Fatalf("la config normalisée devrait produire des cartes: %v", err)
	}
}

func TestShufflePreservesSet(t *testing.T) {
	cards, _ := BuildDeck(DefaultDeckConfig())
	before := map[string]bool{}
	for _, c := range cards {
		before[c.ID] = true
	}
	ShuffleDeck(cards)
	if len(cards) != 52 {
		t.Fatalf("le mélange ne devrait pas changer le nombre de cartes, en a %d", len(cards))
	}
	for _, c := range cards {
		if !before[c.ID] {
			t.Fatalf("carte inattendue après mélange: %s", c.ID)
		}
	}
}
