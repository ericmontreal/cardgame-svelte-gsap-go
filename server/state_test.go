package main

import (
	"testing"
)

// newEngineWithCards crée un moteur avec un sabot de `n` cartes prêtes à être
// manipulées. Les IDs sont "c-0".."c-(n-1)".
func newEngineWithCards(t *testing.T, n int) *engine {
	t.Helper()
	e := newEngine()
	cards := make([]Card, n)
	for i := 0; i < n; i++ {
		cards[i] = Card{ID: idN(i), FaceID: "1_club"}
	}
	e.LoadDeck(cards)
	return e
}

func idN(i int) string {
	return "c-" + itoaSimple(i)
}

// itoaSimple : évite d'importer strconv dans le helper purement utilitaire.
func itoaSimple(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}

func TestLoadDeckInitializesSabot(t *testing.T) {
	e := newEngineWithCards(t, 5)
	if !e.Initialized() {
		t.Fatal("le moteur devrait être initialisé après LoadDeck")
	}
	if got := len(e.sabot); got != 5 {
		t.Fatalf("sabot devrait contenir 5 cartes, en a %d", got)
	}
	// Toutes les cartes sont face cachée dans le sabot.
	for _, c := range e.cards {
		if c.Zone != ZoneSabot {
			t.Fatalf("la carte %s devrait être dans le sabot, zone=%s", c.ID, c.Zone)
		}
		if c.FaceUp {
			t.Fatalf("la carte %s devrait être face cachée dans le sabot", c.ID)
		}
	}
}

func TestFlipTogglesFace(t *testing.T) {
	e := newEngineWithCards(t, 1)
	id, _ := e.DrawSabot(Transfer{Target: TargetTable, X: 10, Y: 10})
	if id == "" {
		t.Fatal("DrawSabot aurait dû tirer une carte")
	}
	c := e.findCard(id)
	if c == nil || !c.FaceUp {
		t.Fatal("une carte tirée sur la table devrait être face visible")
	}
	if !e.Flip(id) {
		t.Fatal("Flip aurait dû réussir")
	}
	if e.findCard(id).FaceUp {
		t.Fatal("la carte devrait être face cachée après flip")
	}
}

func TestFlipRejectedOnSabot(t *testing.T) {
	// Une carte restée dans le sabot ne peut pas être retournée directement.
	e := newEngineWithCards(t, 2)
	id := e.sabot[0]
	if e.Flip(id) {
		t.Fatal("Flip sur une carte de sabot devrait échouer")
	}
}

func TestBringToFrontAndZOrder(t *testing.T) {
	e := newEngineWithCards(t, 3)
	id1, _ := e.DrawSabot(Transfer{Target: TargetTable, X: 1, Y: 1})
	id2, _ := e.DrawSabot(Transfer{Target: TargetTable, X: 2, Y: 2})
	// id2 a été tirée après id1 : elle doit être devant (Z supérieur).
	if e.findCard(id1).Z >= e.findCard(id2).Z {
		t.Fatal("la 2e carte tirée devrait être devant la 1re")
	}
	// BringToFront ramène id1 au premier plan.
	if !e.BringToFront(id1) {
		t.Fatal("BringToFront aurait dû réussir")
	}
	if e.findCard(id1).Z <= e.findCard(id2).Z {
		t.Fatal("id1 devrait désormais avoir le plus grand Z")
	}
}

func TestTransferTableToAvatarHidesCard(t *testing.T) {
	// §6 : une carte déposée sur un avatar disparaît de la zone publique et va
	// dans la main privée du joueur.
	e := newEngineWithCards(t, 1)
	id, _ := e.DrawSabot(Transfer{Target: TargetTable, X: 5, Y: 5})
	res := e.TransferCard(Transfer{CardID: id, Target: TargetAvatar, OwnerID: "u-alice"})
	if !res.PublicChanged || res.HandOwner != "u-alice" {
		t.Fatalf("transfert vers avatar attendu (publicChanged, alice), got %+v", res)
	}
	c := e.findCard(id)
	if c == nil || c.Zone != ZoneHand || c.Owner != "u-alice" {
		t.Fatalf("la carte devrait être dans la main d'alice, got %+v", c)
	}
	// Elle ne doit plus apparaître dans l'état public.
	st := e.snapshotPublic()
	for _, tc := range st.Table {
		if tc.ID == id {
			t.Fatal("la carte transférée ne devrait plus être publique")
		}
	}
	// Mais elle doit apparaître dans la main privée d'alice.
	h := e.snapshotHand("u-alice")
	if len(h.Cards) != 1 || h.Cards[0].ID != id {
		t.Fatalf("alice devrait avoir la carte en main, got %+v", h.Cards)
	}
}

func TestTransferHandToTableAtDropPosition(t *testing.T) {
	// §6 : une carte glissée depuis la main vers le tapis apparaît exactement à
	// la position de relâchement.
	e := newEngineWithCards(t, 1)
	id, _ := e.DrawSabot(Transfer{Target: TargetTable, X: 0, Y: 0})
	e.TransferCard(Transfer{CardID: id, Target: TargetAvatar, OwnerID: "u-alice"})
	res := e.TransferCard(Transfer{CardID: id, Target: TargetTable, X: 123, Y: 456})
	if !res.PublicChanged {
		t.Fatal("le transfert main->table devrait changer l'état public")
	}
	// Alice doit être notifiée que sa main a perdu cette carte, sinon elle
	// reste affichée dans sa main tant qu'aucun autre événement ne la
	// rafraîchit (bug constaté en usage réel).
	if res.FromHandOwner != "u-alice" {
		t.Fatalf("FromHandOwner devrait valoir u-alice, got %q", res.FromHandOwner)
	}
	c := e.findCard(id)
	if c == nil || c.Zone != ZoneTable || c.X != 123 || c.Y != 456 {
		t.Fatalf("la carte devrait être sur la table à (123,456), got %+v", c)
	}
	// La main d'alice doit désormais être vide.
	h := e.snapshotHand("u-alice")
	if len(h.Cards) != 0 {
		t.Fatalf("la main d'alice devrait être vide après le transfert, got %+v", h.Cards)
	}
}

func TestTransferTableToSabot(t *testing.T) {
	e := newEngineWithCards(t, 2)
	id, _ := e.DrawSabot(Transfer{Target: TargetTable, X: 0, Y: 0})
	before := len(e.sabot)
	res := e.TransferCard(Transfer{CardID: id, Target: TargetSabot})
	if !res.PublicChanged {
		t.Fatal("le retour au sabot devrait changer l'état public")
	}
	if len(e.sabot) != before+1 {
		t.Fatalf("le sabot devrait avoir grandi d'une carte, a %d (avant %d)", len(e.sabot), before)
	}
	c := e.findCard(id)
	if c == nil || c.Zone != ZoneSabot || c.FaceUp {
		t.Fatalf("la carte devrait être face cachée dans le sabot, got %+v", c)
	}
}

func TestDrawSabotOrder(t *testing.T) {
	// On tire par le sommet : la dernière carte chargée sort en premier.
	e := newEngineWithCards(t, 3)
	last := e.sabot[len(e.sabot)-1]
	id, _ := e.DrawSabot(Transfer{Target: TargetTable, X: 0, Y: 0})
	if id != last {
		t.Fatalf("on devrait tirer le sommet (%s), a tiré %s", last, id)
	}
	if len(e.sabot) != 2 {
		t.Fatalf("le sabot devrait contenir 2 cartes après tirage, en a %d", len(e.sabot))
	}
}

func TestRotateUpdatesAngle(t *testing.T) {
	e := newEngineWithCards(t, 1)
	id, _ := e.DrawSabot(Transfer{Target: TargetTable, X: 0, Y: 0})
	if !e.Rotate(id, 45) {
		t.Fatal("Rotate aurait dû réussir")
	}
	if e.findCard(id).Rotate != 45 {
		t.Fatal("l'angle de rotation n'a pas été appliqué")
	}
}

func TestSnapshotExcludesPrivateHands(t *testing.T) {
	e := newEngineWithCards(t, 2)
	id, _ := e.DrawSabot(Transfer{Target: TargetTable, X: 0, Y: 0})
	e.TransferCard(Transfer{CardID: id, Target: TargetAvatar, OwnerID: "u-alice"})
	st := e.snapshotPublic()
	// Une carte en main privée ne doit pas fuiter dans l'état public.
	for _, c := range st.Table {
		if c.ID == id {
			t.Fatal("la main privée ne doit pas apparaître dans l'état public")
		}
	}
}

func TestEnsurePlayerAssignsDistinctPositions(t *testing.T) {
	e := newEngine()
	p1 := e.ensurePlayer("u-a", "alice", tableW, tableH)
	p2 := e.ensurePlayer("u-b", "bob", tableW, tableH)
	if p1.AX == p2.AX && p1.AY == p2.AY {
		t.Fatal("deux joueurs devraient avoir des positions d'avatar distinctes")
	}
	// ensurePlayer est idempotent : même userID -> même fiche.
	p1b := e.ensurePlayer("u-a", "alice-renamed", tableW, tableH)
	if p1b != p1 {
		t.Fatal("ensurePlayer devrait retourner la même fiche pour un userID donné")
	}
	if p1.Name != "alice-renamed" {
		t.Fatal("le nom du joueur devrait être mis à jour")
	}
}
