package main

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// ---- Mapping rang/couleur -> faceId du sprite -----------------------------
//
// Le sprite officiel (SVG-cards 2.0.1) expose les groupes :
//   - couleurs : 1..10, jack, queen, king  pour club/diamond/heart/spade
//   - jokers    : black_joker, red_joker (et dos : back)
// Les faceIds attendus côté client sont donc "1_club" ... "king_spade".

var suitOrder = []string{"club", "diamond", "heart", "spade"}

// rankToFrag convertit un rang (1..13) en fragment de faceId du sprite.
//   1  -> "1"   (As affiché "1" dans le sprite svg-cards)
//   11 -> "jack", 12 -> "queen", 13 -> "king"
func rankToFrag(r int) string {
	switch r {
	case 11:
		return "jack"
	case 12:
		return "queen"
	case 13:
		return "king"
	default:
		return strconv.Itoa(r)
	}
}

// fragToRank convertit un fragment de faceId en rang numérique (1..13).
func fragToRank(frag string) (int, bool) {
	switch strings.ToLower(frag) {
	case "a", "ace", "1":
		return 1, true
	case "j", "jack":
		return 11, true
	case "q", "queen":
		return 12, true
	case "k", "king":
		return 13, true
	}
	if n, err := strconv.Atoi(strings.TrimSpace(frag)); err == nil && n >= 2 && n <= 10 {
		return n, true
	}
	return 0, false
}

// faceIDOf construit le faceId d'une carte pour le sprite ("1_club", "king_spade").
func faceIDOf(suit string, rank int) string {
	return fmt.Sprintf("%s_%s", rankToFrag(rank), suit)
}

// ---- Config du menu init (cases à cocher) --------------------------------

// DeckConfig décrit la composition du sabot demandée par le menu d'initialisation.
// Toutes les options sontissues de cases à cocher côté client (§4).
type DeckConfig struct {
	DeckCount int      `json:"deckCount"` // nombre de jeux (1, 2, ...)
	Suits     []string `json:"suits"`     // sous-ensemble de {club,diamond,heart,spade}
	FromRank  int      `json:"fromRank"`  // borne basse incluse (2..13)
	ToRank    int      `json:"toRank"`    // borne haute incluse (2..13)
	Jokers    string   `json:"jokers"`    // "none" | "black" | "red" | "both"
}

// DefaultDeckConfig offre un sabot classique (52 cartes) si rien n'est précisé.
func DefaultDeckConfig() DeckConfig {
	return DeckConfig{
		DeckCount: 1,
		Suits:     []string{"club", "diamond", "heart", "spade"},
		FromRank:  1,
		ToRank:    13,
		Jokers:    "none",
	}
}

// Normalize valide et normalise une config venue du client. En cas de champ
// invalide, on retombe sur une valeur saine. Renvoie une erreur uniquement si
// la config ne produit aucune carte.
func (c DeckConfig) Normalize() (DeckConfig, error) {
	if c.DeckCount < 1 {
		c.DeckCount = 1
	}
	if c.DeckCount > 8 {
		c.DeckCount = 8 // plafond anti-abus (cohérence technique)
	}
	// Couleurs : garder uniquement celles reconnues ; défaut = les 4.
	if len(c.Suits) == 0 {
		c.Suits = suitOrder
	}
	clean := make([]string, 0, len(c.Suits))
	for _, s := range c.Suits {
		if isSuit(s) {
			clean = append(clean, s)
		}
	}
	if len(clean) == 0 {
		clean = suitOrder
	}
	c.Suits = clean
	// Bornes de rangs : au moins 1 et au plus 13, from <= to.
	if c.FromRank < 1 {
		c.FromRank = 1
	}
	if c.ToRank > 13 {
		c.ToRank = 13
	}
	if c.FromRank > c.ToRank {
		c.FromRank, c.ToRank = c.ToRank, c.FromRank
	}
	// Jokers : valeur parmi celles attendues.
	switch c.Jokers {
	case "none", "black", "red", "both":
	default:
		c.Jokers = "none"
	}
	// Vérification finale : la config doit produire au moins une carte.
	if c.DeckCount*len(c.Suits)*(c.ToRank-c.FromRank+1) == 0 {
		return c, fmt.Errorf("deck config ne produit aucune carte")
	}
	return c, nil
}

func isSuit(s string) bool {
	for _, v := range suitOrder {
		if v == s {
			return true
		}
	}
	return false
}

// ---- Construction du sabot ------------------------------------------------

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

// BuildDeck construit la liste des cartes (non mélangées) selon la config.
// L'ordre est stable et déterministe : decks × couleurs × rangs. Le client
// peut demander un mélange via un message "shuffle" séparé ; par défaut le
// sabot est rangé (comme un jeu neuf sorti de la boîte).
func BuildDeck(cfg DeckConfig) ([]Card, error) {
	cfg, err := cfg.Normalize()
	if err != nil {
		return nil, err
	}
	var out []Card
	id := 0
	for d := 0; d < cfg.DeckCount; d++ {
		for _, suit := range cfg.Suits {
			for r := cfg.FromRank; r <= cfg.ToRank; r++ {
				out = append(out, Card{
					ID:     fmt.Sprintf("c-%d", id),
					FaceID: faceIDOf(suit, r),
				})
				id++
			}
		}
		// Jokers : un jeu "both" ajoute un black_joker et un red_joker par deck.
		switch cfg.Jokers {
		case "black":
			out = append(out, Card{ID: fmt.Sprintf("c-%d", id), FaceID: "black_joker"})
			id++
		case "red":
			out = append(out, Card{ID: fmt.Sprintf("c-%d", id), FaceID: "red_joker"})
			id++
		case "both":
			out = append(out, Card{ID: fmt.Sprintf("c-%d", id), FaceID: "black_joker"})
			id++
			out = append(out, Card{ID: fmt.Sprintf("c-%d", id), FaceID: "red_joker"})
			id++
		}
	}
	return out, nil
}

// ShuffleDeck mélange un jeu de cartes (Fisher-Yates) en place.
func ShuffleDeck(cards []Card) {
	rng.Shuffle(len(cards), func(i, j int) {
		cards[i], cards[j] = cards[j], cards[i]
	})
}
