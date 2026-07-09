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
	if c == nil || c.FaceUp {
		t.Fatal("une carte tirée du sabot directement sur la table devrait rester face cachée")
	}
	ok, handOwner := e.Flip(id)
	if !ok {
		t.Fatal("Flip aurait dû réussir")
	}
	if handOwner != "" {
		t.Fatalf("une carte de table ne devrait pas avoir de handOwner, got %q", handOwner)
	}
	if !e.findCard(id).FaceUp {
		t.Fatal("la carte devrait être face visible après flip")
	}
}

func TestFlipRejectedOnSabot(t *testing.T) {
	// Une carte restée dans le sabot ne peut pas être retournée directement.
	e := newEngineWithCards(t, 2)
	id := e.sabot[0]
	if ok, _ := e.Flip(id); ok {
		t.Fatal("Flip sur une carte de sabot devrait échouer")
	}
}

func TestFlipOnHandCardReportsOwner(t *testing.T) {
	// Une carte retournée dans une main privée n'apparaît jamais dans l'état
	// public (snapshotPublic l'exclut) : Flip doit donc signaler le
	// propriétaire à notifier directement, sinon il ne voit jamais le
	// résultat de son propre flip.
	e := newEngineWithCards(t, 1)
	id, _ := e.DrawSabot(Transfer{Target: TargetAvatar, OwnerID: "u-alice"})
	ok, handOwner := e.Flip(id)
	if !ok {
		t.Fatal("Flip aurait dû réussir")
	}
	if handOwner != "u-alice" {
		t.Fatalf("handOwner attendu u-alice, got %q", handOwner)
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

func TestTransferPreservesFaceState(t *testing.T) {
	// Un simple déplacement (drag) ne doit jamais changer l'état face d'une
	// carte : seule une distribution depuis le sabot VERS UNE MAIN (DrawSabot,
	// TargetAvatar/TargetHand) la révèle ; un dépôt sur la table ne révèle
	// jamais, qu'il vienne d'un DnD manuel ou d'un tirage direct sabot→table.
	// Un joueur doit pouvoir retourner une carte de sa main avant de la poser
	// sur le tapis, et ce choix doit être respecté.
	e := newEngineWithCards(t, 1)
	id, _ := e.DrawSabot(Transfer{Target: TargetAvatar, OwnerID: "u-alice"}) // dealt=true -> face visible
	if !e.findCard(id).FaceUp {
		t.Fatal("une carte distribuée dans une main devrait être face visible")
	}
	if ok, _ := e.Flip(id); !ok { // alice retourne la carte face cachée dans sa main
		t.Fatal("Flip aurait dû réussir")
	}
	if e.findCard(id).FaceUp {
		t.Fatal("la carte devrait être face cachée après Flip")
	}
	e.TransferCard(Transfer{CardID: id, Target: TargetTable, X: 10, Y: 20})
	if e.findCard(id).FaceUp {
		t.Fatal("poser une carte face cachée sur le tapis ne devrait pas la révéler")
	}
}

func TestDrawSabotRevealsOnlyWhenGivenToAPlayer(t *testing.T) {
	// Une carte n'est révélée que lorsqu'elle est donnée à un joueur (avatar/
	// main), jamais lors d'un simple dépôt sur la table : un tirage direct du
	// sabot vers le tapis doit rester face cachée (comme un vrai jeu où l'on
	// ne retourne pas automatiquement les cartes posées).
	e := newEngineWithCards(t, 2)
	idTable, _ := e.DrawSabot(Transfer{Target: TargetTable, X: 0, Y: 0})
	if e.findCard(idTable).FaceUp {
		t.Fatal("une carte tirée du sabot directement sur la table devrait rester face cachée")
	}
	idHand, _ := e.DrawSabot(Transfer{Target: TargetAvatar, OwnerID: "u-bob"})
	if !e.findCard(idHand).FaceUp {
		t.Fatal("une carte distribuée dans une main devrait être face visible pour son propriétaire")
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

func TestSnapshotReportsHandCountWithoutLeakingCards(t *testing.T) {
	// Les autres joueurs doivent voir combien de cartes chacun a en main
	// (comme pour le sabot), sans jamais voir les cartes elles-mêmes.
	e := newEngineWithCards(t, 3)
	e.ensurePlayer("u-alice", "alice", tableW, tableH)
	e.ensurePlayer("u-bob", "bob", tableW, tableH)
	id1, _ := e.DrawSabot(Transfer{Target: TargetAvatar, OwnerID: "u-alice"})
	id2, _ := e.DrawSabot(Transfer{Target: TargetAvatar, OwnerID: "u-alice"})
	_, _ = e.DrawSabot(Transfer{Target: TargetAvatar, OwnerID: "u-bob"})

	st := e.snapshotPublic()
	counts := map[string]int{}
	for _, p := range st.Players {
		counts[p.UserID] = p.HandCount
	}
	if counts["u-alice"] != 2 {
		t.Fatalf("alice devrait avoir 2 cartes en main, got %d", counts["u-alice"])
	}
	if counts["u-bob"] != 1 {
		t.Fatalf("bob devrait avoir 1 carte en main, got %d", counts["u-bob"])
	}
	for _, c := range st.Table {
		if c.ID == id1 || c.ID == id2 {
			t.Fatal("les cartes en main ne doivent jamais apparaître dans l'état public")
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

func TestEnsurePlayerKeepsDistinctSeatAfterReconnect(t *testing.T) {
	// Bug constaté en usage réel : après déconnexion puis reconnexion, un
	// joueur reprenait le même siège qu'un autre joueur resté connecté (le
	// siège dépendait de len(e.players), qui redescend au départ). Les deux
	// avatars se superposaient alors exactement à l'écran.
	e := newEngine()
	e.ensurePlayer("u-alice", "alice", tableW, tableH)
	pBob := e.ensurePlayer("u-bob", "bob", tableW, tableH)
	e.removePlayer("u-alice") // alice se déconnecte ; bob reste seul

	pAliceAgain := e.ensurePlayer("u-alice", "alice", tableW, tableH)
	if pAliceAgain.AX == pBob.AX && pAliceAgain.AY == pBob.AY {
		t.Fatalf("alice ne devrait pas reprendre le siège de bob après reconnexion: alice=%+v bob=%+v", pAliceAgain, pBob)
	}
}

// ---- Mutations par lot (sélection multiple) --------------------------------

func TestMoveManyPreservesRelativeZOrder(t *testing.T) {
	// Un groupe déplacé d'un bloc passe au premier plan, mais l'ordre Z
	// RELATIF de ses cartes doit être préservé (une pile déplacée reste la
	// même pile), quel que soit l'ordre des items dans le payload client.
	e := newEngineWithCards(t, 3)
	var ids []string
	for i := 0; i < 3; i++ {
		id, _ := e.DrawSabot(Transfer{Target: TargetTable, X: float64(10 * i), Y: 0})
		ids = append(ids, id)
	}
	// ids[0] a le Z le plus bas, ids[2] le plus haut (tirages successifs).
	zBefore := map[string]int{}
	for _, id := range ids {
		zBefore[id] = e.findCard(id).Z
	}
	// Payload volontairement dans un ordre différent de l'ordre Z.
	ok := e.MoveMany([]CardMove{
		{CardID: ids[2], X: 300, Y: 300},
		{CardID: ids[0], X: 100, Y: 100},
		{CardID: ids[1], X: 200, Y: 200},
	})
	if !ok {
		t.Fatal("MoveMany devrait signaler un changement")
	}
	c0, c1, c2 := e.findCard(ids[0]), e.findCard(ids[1]), e.findCard(ids[2])
	if c0.X != 100 || c1.X != 200 || c2.X != 300 {
		t.Fatalf("positions inattendues: %v %v %v", c0.X, c1.X, c2.X)
	}
	// L'ordre relatif d'avant (c0 < c1 < c2) doit être conservé après.
	if !(c0.Z < c1.Z && c1.Z < c2.Z) {
		t.Fatalf("ordre Z relatif non préservé: z0=%d z1=%d z2=%d (avant: %v)", c0.Z, c1.Z, c2.Z, zBefore)
	}
	// Et le groupe entier est passé au premier plan.
	for _, id := range ids {
		if e.findCard(id).Z <= zBefore[id] {
			t.Fatalf("la carte %s devrait avoir un Z supérieur à avant", id)
		}
	}
}

func TestMoveManyIgnoresUnknownAndNonTableCards(t *testing.T) {
	e := newEngineWithCards(t, 2)
	id, _ := e.DrawSabot(Transfer{Target: TargetTable, X: 5, Y: 5})
	sabotTop := e.sabot[len(e.sabot)-1]
	// Lot mixte : une carte valide, une inconnue, une encore au sabot.
	ok := e.MoveMany([]CardMove{
		{CardID: "c-inconnue", X: 50, Y: 50},
		{CardID: sabotTop, X: 60, Y: 60},
		{CardID: id, X: 70, Y: 70},
	})
	if !ok {
		t.Fatal("MoveMany devrait signaler un changement (une carte valide)")
	}
	if c := e.findCard(id); c.X != 70 || c.Y != 70 {
		t.Fatalf("la carte de table devrait avoir bougé, got (%v,%v)", c.X, c.Y)
	}
	if c := e.findCard(sabotTop); c.Zone != ZoneSabot || c.X != 0 {
		t.Fatalf("une carte du sabot ne doit jamais être déplacée par MoveMany, got %+v", c)
	}
	// Lot entièrement invalide : aucun changement signalé.
	if e.MoveMany([]CardMove{{CardID: "c-inconnue", X: 1, Y: 1}}) {
		t.Fatal("MoveMany ne devrait rien signaler pour un lot invalide")
	}
}

func TestFlipManyFlipsOncePerCardAndReportsHandOwners(t *testing.T) {
	e := newEngineWithCards(t, 3)
	onTable, _ := e.DrawSabot(Transfer{Target: TargetTable, X: 5, Y: 5})
	inHand, _ := e.DrawSabot(Transfer{Target: TargetHand, OwnerID: "u-alice"})
	sabotTop := e.sabot[len(e.sabot)-1]

	// L'ID de table est dupliqué : il ne doit être retourné qu'UNE fois
	// (sinon le double flip est un no-op silencieux et trompeur).
	changed, owners := e.FlipMany([]string{onTable, onTable, inHand, sabotTop})
	if !changed {
		t.Fatal("FlipMany devrait signaler un changement")
	}
	if c := e.findCard(onTable); !c.FaceUp {
		t.Fatal("la carte de table devrait être face visible après un seul flip")
	}
	if c := e.findCard(inHand); c.FaceUp {
		t.Fatal("la carte en main (distribuée face visible) devrait être retournée face cachée")
	}
	if !owners["u-alice"] || len(owners) != 1 {
		t.Fatalf("alice devrait être notifiée (sa main a changé), got %v", owners)
	}
	if c := e.findCard(sabotTop); c.FaceUp {
		t.Fatal("une carte du sabot ne doit jamais être retournée")
	}
	// Lot sans aucune carte retournable : aucun changement.
	changed, _ = e.FlipMany([]string{sabotTop, "c-inconnue"})
	if changed {
		t.Fatal("FlipMany ne devrait rien signaler pour un lot invalide")
	}
}

func TestTransferManyToHandAggregatesOwners(t *testing.T) {
	e := newEngineWithCards(t, 3)
	id1, _ := e.DrawSabot(Transfer{Target: TargetTable, X: 10, Y: 10})
	id2, _ := e.DrawSabot(Transfer{Target: TargetTable, X: 20, Y: 20})
	res := e.TransferMany([]string{id1, id2}, TargetHand, "u-alice")
	if !res.PublicChanged {
		t.Fatal("deux cartes ont quitté la table : l'état public devrait changer")
	}
	if !res.HandOwners["u-alice"] || len(res.HandOwners) != 1 {
		t.Fatalf("alice devrait être l'unique main notifiée, got %v", res.HandOwners)
	}
	if h := e.snapshotHand("u-alice"); len(h.Cards) != 2 {
		t.Fatalf("la main d'alice devrait contenir 2 cartes, en a %d", len(h.Cards))
	}
	if st := e.snapshotPublic(); len(st.Table) != 0 {
		t.Fatalf("la table devrait être vide, contient %d cartes", len(st.Table))
	}
}

func TestTransferManyToSabotPreservesPileOrder(t *testing.T) {
	// Une pile remise au sabot doit garder son ordre : la carte du dessus de
	// la pile (Z max) finit au sommet du sabot, même si le payload client
	// l'envoie en premier.
	e := newEngineWithCards(t, 2)
	bottom, _ := e.DrawSabot(Transfer{Target: TargetTable, X: 10, Y: 10})
	top, _ := e.DrawSabot(Transfer{Target: TargetTable, X: 10, Y: 10})
	e.findCard(top).FaceUp = true // sera remise face cachée par le sabot
	res := e.TransferMany([]string{top, bottom}, TargetSabot, "")
	if !res.PublicChanged {
		t.Fatal("l'état public devrait changer")
	}
	if n := len(e.sabot); n != 2 {
		t.Fatalf("le sabot devrait contenir 2 cartes, en a %d", n)
	}
	if got := e.sabot[len(e.sabot)-1]; got != top {
		t.Fatalf("la carte du dessus de la pile devrait être au sommet du sabot, got %s (attendu %s)", got, top)
	}
	if c := e.findCard(top); c.FaceUp {
		t.Fatal("une carte remise au sabot doit être face cachée")
	}
}

func TestTransferManyRejectsTableTargetAndSabotCards(t *testing.T) {
	e := newEngineWithCards(t, 2)
	id, _ := e.DrawSabot(Transfer{Target: TargetTable, X: 10, Y: 10})
	sabotTop := e.sabot[len(e.sabot)-1]

	// Cible table : refusée (les positions individuelles passent par MoveMany).
	res := e.TransferMany([]string{id}, TargetTable, "")
	if res.PublicChanged {
		t.Fatal("TransferMany vers la table devrait être refusé")
	}
	if c := e.findCard(id); c.Zone != ZoneTable {
		t.Fatal("la carte devrait être restée sur la table")
	}
	// Une carte encore au sabot est ignorée (son retrait relève de DrawSabot).
	res = e.TransferMany([]string{sabotTop, id}, TargetHand, "u-bob")
	if c := e.findCard(sabotTop); c.Zone != ZoneSabot {
		t.Fatal("une carte du sabot ne doit pas être transférée par TransferMany")
	}
	if len(e.sabot) != 1 {
		t.Fatalf("le sabot ne devrait pas avoir changé, taille=%d", len(e.sabot))
	}
	if h := e.snapshotHand("u-bob"); len(h.Cards) != 1 {
		t.Fatalf("seule la carte de table devrait être dans la main de bob, got %d", len(h.Cards))
	}
	_ = res
}
