package main

import (
	"math"
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
	// carte : seul le joueur la retourne (double-clic, Flip). La face qu'il a
	// choisie dans sa main doit être conservée quand il la pose sur le tapis,
	// dans un sens comme dans l'autre.
	e := newEngineWithCards(t, 2)
	id, _ := e.DrawSabot(Transfer{Target: TargetAvatar, OwnerID: "u-alice"}) // arrive face cachée
	if ok, _ := e.Flip(id); !ok || !e.findCard(id).FaceUp { // alice la retourne face visible
		t.Fatal("Flip aurait dû rendre la carte face visible")
	}
	e.TransferCard(Transfer{CardID: id, Target: TargetTable, X: 10, Y: 20})
	if !e.findCard(id).FaceUp {
		t.Fatal("poser une carte face visible sur le tapis ne doit pas la cacher")
	}
	id2, _ := e.DrawSabot(Transfer{Target: TargetHand, OwnerID: "u-alice"}) // face cachée, non retournée
	e.TransferCard(Transfer{CardID: id2, Target: TargetTable, X: 30, Y: 20})
	if e.findCard(id2).FaceUp {
		t.Fatal("poser une carte face cachée sur le tapis ne doit pas la révéler")
	}
}

func TestCardsKeepTheirFaceWhenReturnedToTheSabot(t *testing.T) {
	// Aucun retournement automatique, dans un sens comme dans l'autre : une
	// carte face visible rendue au sabot y reste visible, et l'état public
	// l'expose comme carte du dessus ; tirée à nouveau, elle sort visible.
	e := newEngineWithCards(t, 4)
	id, _ := e.DrawSabot(Transfer{Target: TargetTable, X: 300, Y: 300})
	e.Flip(id) // face visible
	if e.snapshotPublic().SabotTop != nil {
		t.Fatal("le sabot n'expose pas une carte du dessus face cachée")
	}
	e.TransferCard(Transfer{CardID: id, Target: TargetSabot})
	if !e.findCard(id).FaceUp {
		t.Fatal("une carte face visible rendue au sabot doit rester face visible")
	}
	if top := e.snapshotPublic().SabotTop; top == nil || top.ID != id {
		t.Fatal("l'état public doit exposer la carte du dessus face visible")
	}
	again, _ := e.DrawSabot(Transfer{Target: TargetTable, X: 200, Y: 200})
	if again != id || !e.findCard(again).FaceUp {
		t.Fatal("retirée du sabot, la carte doit ressortir face visible")
	}
	// « Au fond du sabot » conserve aussi la face ; l'abandon, lui, cache
	// (promesse de la fenêtre de départ).
	e.TransferManyToSabotBottom([]string{again})
	if !e.findCard(again).FaceUp {
		t.Fatal("« Au fond du sabot » doit conserver la face")
	}
	e.ensurePlayer("u-alice", "alice", tableW, tableH)
	h, _ := e.DrawSabot(Transfer{Target: TargetHand, OwnerID: "u-alice"})
	e.Flip(h)
	e.Abandon("u-alice", AbandonToSabot)
	if e.findCard(h).FaceUp {
		t.Fatal("la main d'un partant doit partir face cachée")
	}
}

func TestHandFaceUpOptionTurnsCardsEnteringAHandFaceUp(t *testing.T) {
	e := newEngineWithCards(t, 4)
	e.ensurePlayer("u-alice", "alice", tableW, tableH)
	e.ensurePlayer("u-bob", "bob", tableW, tableH)
	e.SetHandFaceUp(true)
	// Depuis le sabot : face visible en main.
	a, _ := e.DrawSabot(Transfer{Target: TargetHand, OwnerID: "u-alice"})
	if !e.findCard(a).FaceUp {
		t.Fatal("avec l'option, une carte tirée dans une main doit y être face visible")
	}
	// Le joueur peut la retourner, et la reposer dans sa main ne la retourne pas.
	e.Flip(a)
	e.TransferCard(Transfer{CardID: a, Target: TargetAvatar, OwnerID: "u-alice"})
	if e.findCard(a).FaceUp {
		t.Fatal("une carte reposée dans la main qui la tenait garde la face choisie par son joueur")
	}
	// Donnée à un autre joueur : elle entre dans sa main, face visible.
	e.TransferCard(Transfer{CardID: a, Target: TargetAvatar, OwnerID: "u-bob"})
	if !e.findCard(a).FaceUp {
		t.Fatal("une carte qui entre dans la main d'un autre joueur doit y être face visible")
	}
	// Depuis le tapis, face cachée : face visible en main.
	b, _ := e.DrawSabot(Transfer{Target: TargetTable, X: 300, Y: 300})
	e.TransferCard(Transfer{CardID: b, Target: TargetHand, OwnerID: "u-alice"})
	if !e.findCard(b).FaceUp {
		t.Fatal("une carte ramassée du tapis doit être face visible en main")
	}
	// Option désactivée : retour à la règle générale, aucun retournement.
	e.SetHandFaceUp(false)
	c, _ := e.DrawSabot(Transfer{Target: TargetHand, OwnerID: "u-alice"})
	if e.findCard(c).FaceUp {
		t.Fatal("sans l'option, une carte tirée en main reste face cachée")
	}
}

func TestDrawSabotNeverRevealsTheCard(t *testing.T) {
	// Aucun retournement automatique : seul un double-clic retourne une carte.
	// Tirée sur le tapis ou distribuée dans une main, elle reste face cachée.
	e := newEngineWithCards(t, 3)
	e.ensurePlayer("u-bob", "bob", tableW, tableH)
	idTable, _ := e.DrawSabot(Transfer{Target: TargetTable, X: 0, Y: 0})
	if e.findCard(idTable).FaceUp {
		t.Fatal("une carte tirée du sabot sur la table doit rester face cachée")
	}
	idHand, _ := e.DrawSabot(Transfer{Target: TargetAvatar, OwnerID: "u-bob"})
	if e.findCard(idHand).FaceUp {
		t.Fatal("une carte distribuée dans une main doit arriver face cachée")
	}
	// Son propriétaire la retourne lui-même.
	if ok, owner := e.Flip(idHand); !ok || owner != "u-bob" || !e.findCard(idHand).FaceUp {
		t.Fatal("le propriétaire doit pouvoir retourner la carte de sa main")
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

func TestEnsurePlayerRestoresSameSeatAfterRefresh(t *testing.T) {
	// Deuxième bug constaté en usage réel, inverse du précédent : rafraîchir sa
	// page déplaçait le joueur autour de la table. Fermer l'onglet ferme la
	// WebSocket, le Player est retiré, et le siège était alors pris sur un
	// compteur qui ne faisait que croître — six rafraîchissements faisaient le
	// tour complet.
	e := newEngine()
	e.ensurePlayer("u-alice", "alice", tableW, tableH)
	pBob := e.ensurePlayer("u-bob", "bob", tableW, tableH)

	// Bob rafraîchit sa page : depart puis retour immediat.
	e.removePlayer("u-bob")
	pBobAgain := e.ensurePlayer("u-bob", "bob", tableW, tableH)

	if pBobAgain.AX != pBob.AX || pBobAgain.AY != pBob.AY {
		t.Fatalf("bob devrait retrouver sa chaise après un rafraîchissement: avant=(%v,%v) après=(%v,%v)",
			pBob.AX, pBob.AY, pBobAgain.AX, pBobAgain.AY)
	}

	// Et cela doit tenir sur plusieurs rafraîchissements d'affilée : c'est la
	// répétition qui rendait l'ancien défaut spectaculaire.
	for i := 0; i < 8; i++ {
		e.removePlayer("u-bob")
		p := e.ensurePlayer("u-bob", "bob", tableW, tableH)
		if p.AX != pBob.AX || p.AY != pBob.AY {
			t.Fatalf("rafraîchissement %d : bob a change de chaise (%v,%v) -> (%v,%v)",
				i+1, pBob.AX, pBob.AY, p.AX, p.AY)
		}
	}
}

func TestFreeSeatDoesNotReuseAnAbsentPlayerSeat(t *testing.T) {
	// Un joueur absent garde sa chaise : un NOUVEAU venu qui arrive pendant ce
	// temps doit prendre un siège libre, pas celui qui est reserve. Sinon le
	// joueur parti retrouve sa place occupee, et les avatars se superposent.
	e := newEngine()
	pAlice := e.ensurePlayer("u-alice", "alice", tableW, tableH)
	e.removePlayer("u-alice")

	pCarol := e.ensurePlayer("u-carol", "carol", tableW, tableH)
	if pCarol.AX == pAlice.AX && pCarol.AY == pAlice.AY {
		t.Fatal("carol ne devrait pas hériter du siège reservé à alice")
	}

	pAliceAgain := e.ensurePlayer("u-alice", "alice", tableW, tableH)
	if pAliceAgain.AX != pAlice.AX || pAliceAgain.AY != pAlice.AY {
		t.Fatal("alice devrait retrouver son siège malgré l'arrivée de carol")
	}
}

// ---- Abandon de partie -----------------------------------------------------

// handOf compte les cartes en main d'un joueur.
func handOf(e *engine, userID string) int {
	n := 0
	for _, c := range e.cards {
		if c.Zone == ZoneHand && c.Owner == userID {
			n++
		}
	}
	return n
}

func TestAbandonToTableLeavesCardsFaceDownOnFelt(t *testing.T) {
	e := newEngineWithCards(t, 6)
	p := e.ensurePlayer("u-alice", "alice", tableW, tableH)
	// Trois cartes distribuées à alice, révélées par la distribution.
	for i := 0; i < 3; i++ {
		e.DrawSabot(Transfer{Target: TargetHand, OwnerID: "u-alice"})
	}
	if handOf(e, "u-alice") != 3 {
		t.Fatalf("préparation: alice devrait avoir 3 cartes, en a %d", handOf(e, "u-alice"))
	}

	if !e.Abandon("u-alice", AbandonToTable) {
		t.Fatal("Abandon devrait réussir pour un joueur à table")
	}
	if handOf(e, "u-alice") != 0 {
		t.Fatal("la main du partant doit être vidée")
	}

	onTable := 0
	for _, c := range e.cards {
		if c.Zone != ZoneTable {
			continue
		}
		onTable++
		// Un abandon ne révèle jamais le jeu du partant, même si la carte était
		// face visible dans sa main.
		if c.FaceUp {
			t.Fatalf("carte %s posée face visible lors d'un abandon", c.ID)
		}
		// Elle doit rester entièrement sur le feutre, chaise comprise : les
		// sièges sont calculés sur le pourtour bois, hors zone de pose.
		if c.X < minCardX || c.X > maxCardX || c.Y < minCardY || c.Y > maxCardY {
			t.Fatalf("carte %s hors du feutre: (%v,%v)", c.ID, c.X, c.Y)
		}
	}
	if onTable != 3 {
		t.Fatalf("3 cartes devraient être sur le tapis, %d trouvées", onTable)
	}
	if !p.Abandoned {
		t.Fatal("le joueur devrait être marqué comme parti (chaise grisée)")
	}
}

func TestAbandonToSabotBuriesCardsAtTheBottom(t *testing.T) {
	e := newEngineWithCards(t, 5)
	e.ensurePlayer("u-alice", "alice", tableW, tableH)
	drawn, _ := e.DrawSabot(Transfer{Target: TargetHand, OwnerID: "u-alice"})
	remaining := len(e.sabot)

	if !e.Abandon("u-alice", AbandonToSabot) {
		t.Fatal("Abandon devrait réussir")
	}
	if len(e.sabot) != remaining+1 {
		t.Fatalf("le sabot devrait regagner une carte: %d -> %d", remaining, len(e.sabot))
	}
	// e.sabot va du FOND (index 0) vers le sommet : la carte rendue doit être
	// au fond, donc ressortir en dernier.
	if e.sabot[0] != drawn {
		t.Fatalf("la carte rendue devrait être au fond du sabot, trouvé %q en fond", e.sabot[0])
	}
	if c := e.findCard(drawn); c.FaceUp {
		t.Fatal("une carte remise au sabot doit être face cachée")
	}
}

func TestAbandonedPlayerCannotReceiveCards(t *testing.T) {
	// Donner une carte a un joueur parti la ferait disparaitre du tapis sans
	// que personne puisse la reprendre.
	e := newEngineWithCards(t, 4)
	e.ensurePlayer("u-alice", "alice", tableW, tableH)
	e.ensurePlayer("u-bob", "bob", tableW, tableH)
	e.Abandon("u-bob", AbandonToSabot)

	id, _ := e.DrawSabot(Transfer{Target: TargetTable, X: 300, Y: 300})
	res := e.TransferCard(Transfer{CardID: id, Target: TargetAvatar, OwnerID: "u-bob"})
	if res.HandOwner != "" || handOf(e, "u-bob") != 0 {
		t.Fatal("un joueur parti ne doit pas pouvoir recevoir de carte")
	}
	if c := e.findCard(id); c.Zone != ZoneTable {
		t.Fatalf("la carte refusée doit rester sur le tapis, zone=%v", c.Zone)
	}
}

func TestAbandonedChairSurvivesDisconnectButNotANewGame(t *testing.T) {
	e := newEngineWithCards(t, 4)
	e.ensurePlayer("u-alice", "alice", tableW, tableH)
	e.Abandon("u-alice", AbandonToTable)

	// La fermeture de WebSocket suit immédiatement l'abandon : la chaise grisée
	// doit y survivre, sans quoi le depart serait indistinguable d'une absence.
	e.removePlayer("u-alice")
	if p, ok := e.players["u-alice"]; !ok || !p.Abandoned {
		t.Fatal("la chaise grisée doit survivre à la déconnexion du partant")
	}

	// Une nouvelle partie, elle, remet la table a neuf.
	cards, _ := BuildDeck(DefaultDeckConfig())
	e.LoadDeck(cards)
	if _, ok := e.players["u-alice"]; ok {
		t.Fatal("une nouvelle partie doit effacer les chaises grisees")
	}
}

func TestTransferReportsWhereAReceivedCardCameFrom(t *testing.T) {
	e := newEngineWithCards(t, 6)
	e.ensurePlayer("u-alice", "alice", tableW, tableH)
	e.ensurePlayer("u-bob", "bob", tableW, tableH)

	id, res := e.DrawSabot(Transfer{Target: TargetAvatar, OwnerID: "u-bob"})
	if res.Received != ZoneSabot || res.HandOwner != "u-bob" {
		t.Fatalf("tirage vers bob: reçu=%q destinataire=%q", res.Received, res.HandOwner)
	}
	// De la main de bob à celle d'alice : la carte change de main, et les
	// comptes de cartes en main (publics) doivent être rediffusés.
	if r := e.TransferCard(Transfer{CardID: id, Target: TargetAvatar, OwnerID: "u-alice"}); r.Received != ZoneHand || !r.PublicChanged {
		t.Fatalf("don de main à main: reçu=%q public=%v, attendu %q et true", r.Received, r.PublicChanged, ZoneHand)
	}
	// Alice la lâche sur son propre avatar : elle ne fait que la reposer.
	if r := e.TransferCard(Transfer{CardID: id, Target: TargetAvatar, OwnerID: "u-alice"}); r.Received != "" || r.PublicChanged {
		t.Fatalf("reposer une carte dans sa propre main ne change rien, reçu=%q public=%v", r.Received, r.PublicChanged)
	}
	tbl, _ := e.DrawSabot(Transfer{Target: TargetTable, X: 300, Y: 300})
	if r := e.TransferCard(Transfer{CardID: tbl, Target: TargetHand, OwnerID: "u-alice"}); r.Received != ZoneTable {
		t.Fatalf("ramassage du tapis: reçu=%q, attendu %q", r.Received, ZoneTable)
	}
	// Une carte posée sur le tapis n'entre dans aucune main.
	if r := e.TransferCard(Transfer{CardID: tbl, Target: TargetTable, X: 200, Y: 200}); r.Received != "" {
		t.Fatalf("une pose sur le tapis n'est pas une réception, reçu=%q", r.Received)
	}
}

func TestTransferManyCountsReceivedCardsByOrigin(t *testing.T) {
	e := newEngineWithCards(t, 6)
	e.ensurePlayer("u-alice", "alice", tableW, tableH)
	e.ensurePlayer("u-bob", "bob", tableW, tableH)
	var ids []string
	for i := 0; i < 3; i++ {
		id, _ := e.DrawSabot(Transfer{Target: TargetTable, X: float64(100 + 10*i), Y: 200})
		ids = append(ids, id)
	}
	res := e.TransferMany(ids, TargetAvatar, "u-bob")
	if res.Received[ZoneTable] != 3 || len(res.Received) != 1 {
		t.Fatalf("3 cartes du tapis attendues pour bob, obtenu %v", res.Received)
	}
	// Vers le sabot, personne ne reçoit rien.
	if res := e.TransferMany(ids, TargetSabot, ""); len(res.Received) != 0 {
		t.Fatalf("un retour au sabot n'est pas une réception, obtenu %v", res.Received)
	}
}

func TestHandListsCardsInArrivalOrder(t *testing.T) {
	// Le sabot rend c-5, c-4, c-3... (sommet = fin de e.sabot). On tire c-5
	// et c-4 en main, on pose c-3 sur le tapis puis on la ramasse : la main
	// doit lister c-5, c-4, c-3 — l'ordre d'arrivée, pas celui du paquet.
	e := newEngineWithCards(t, 6)
	e.ensurePlayer("u-alice", "alice", tableW, tableH)
	a, _ := e.DrawSabot(Transfer{Target: TargetHand, OwnerID: "u-alice"})
	b, _ := e.DrawSabot(Transfer{Target: TargetHand, OwnerID: "u-alice"})
	c, _ := e.DrawSabot(Transfer{Target: TargetTable, X: 300, Y: 300})
	e.TransferCard(Transfer{CardID: c, Target: TargetHand, OwnerID: "u-alice"})
	got := e.snapshotHand("u-alice").Cards
	if len(got) != 3 || got[0].ID != a || got[1].ID != b || got[2].ID != c {
		t.Fatalf("ordre d'arrivée attendu %s %s %s, obtenu %v", a, b, c, got)
	}
	// Reposer une carte dans sa propre main ne la déplace pas.
	e.TransferCard(Transfer{CardID: a, Target: TargetAvatar, OwnerID: "u-alice"})
	if got := e.snapshotHand("u-alice").Cards; got[0].ID != a {
		t.Fatalf("une carte reposée dans sa main doit garder sa place, obtenu %v", got)
	}
}

func TestReorderHandFollowsThePlayersChoice(t *testing.T) {
	e := newEngineWithCards(t, 6)
	e.ensurePlayer("u-alice", "alice", tableW, tableH)
	e.ensurePlayer("u-bob", "bob", tableW, tableH)
	a, _ := e.DrawSabot(Transfer{Target: TargetHand, OwnerID: "u-alice"})
	b, _ := e.DrawSabot(Transfer{Target: TargetHand, OwnerID: "u-alice"})
	c, _ := e.DrawSabot(Transfer{Target: TargetHand, OwnerID: "u-alice"})
	bobs, _ := e.DrawSabot(Transfer{Target: TargetHand, OwnerID: "u-bob"})

	// Alice range c, a, b — et tente d'y glisser une carte de Bob et un ID
	// inconnu, qui doivent être ignorés.
	if !e.ReorderHand("u-alice", []string{c, bobs, "c-999", a, b, a}) {
		t.Fatal("le rangement aurait dû s'appliquer")
	}
	got := e.snapshotHand("u-alice").Cards
	if len(got) != 3 || got[0].ID != c || got[1].ID != a || got[2].ID != b {
		t.Fatalf("ordre choisi c a b attendu, obtenu %v", got)
	}
	if h := e.snapshotHand("u-bob").Cards; len(h) != 1 || h[0].ID != bobs {
		t.Fatalf("la main de Bob ne doit pas être touchée, obtenu %v", h)
	}
	// Une carte absente de la liste (arrivée entre-temps) passe à droite.
	d, _ := e.DrawSabot(Transfer{Target: TargetHand, OwnerID: "u-alice"})
	e.ReorderHand("u-alice", []string{b, a, c})
	got = e.snapshotHand("u-alice").Cards
	if got[0].ID != b || got[1].ID != a || got[2].ID != c || got[3].ID != d {
		t.Fatalf("ordre b a c puis %s attendu, obtenu %v", d, got)
	}
	// Rien de valide dans la liste : rien à faire.
	if e.ReorderHand("u-alice", []string{bobs}) {
		t.Fatal("une liste sans carte de la main ne doit rien ranger")
	}
}

func TestSeatsCenterTheAvatarOnTheFeltEdge(t *testing.T) {
	// Le centre du cercle de chaque avatar est sur le bord du feutre : sud et
	// nord à feltInset du bord haut/bas, petits côtés à feltInset du bord
	// gauche/droit. Le cercle empiète ainsi de moitié sur le feutre.
	e := newEngine()
	seat := func(i int) *Player { p := &Player{}; e.layoutAvatar(p, i, tableW, tableH); return p }
	near := func(a, b float64) bool { return math.Abs(a-b) < 0.01 }
	if s := seat(0); !near(s.AY, tableH-feltInset) {
		t.Fatalf("siège sud attendu à y=%.0f, obtenu %.2f", tableH-feltInset, s.AY)
	}
	if n := seat(3); !near(n.AY, feltInset) {
		t.Fatalf("siège nord attendu à y=%.0f, obtenu %.2f", feltInset, n.AY)
	}
	for _, i := range []int{1, 2} {
		if l := seat(i); math.Abs(l.AX-feltInset) > 0.1 {
			t.Fatalf("siège %d (côté gauche) attendu à x=%.0f, obtenu %.2f", i, feltInset, l.AX)
		}
	}
	for _, i := range []int{4, 5} {
		if r := seat(i); math.Abs(r.AX-(tableW-feltInset)) > 0.1 {
			t.Fatalf("siège %d (côté droit) attendu à x=%.0f, obtenu %.2f", i, tableW-feltInset, r.AX)
		}
	}
}

func TestMoveSeatKeepsTheAvatarOnTheFeltEdgeAndRemembersIt(t *testing.T) {
	e := newEngine()
	e.ensurePlayer("u-alice", "alice", tableW, tableH)
	// Lâché au milieu du tapis, près du bord haut : ramené sur le bord haut.
	if !e.MoveSeat("u-alice", 500, feltInset+30) {
		t.Fatal("le déplacement aurait dû s'appliquer")
	}
	p := e.players["u-alice"]
	if p.AX != 500 || p.AY != feltInset {
		t.Fatalf("attendu (500, %.0f), obtenu (%.1f, %.1f)", feltInset, p.AX, p.AY)
	}
	// Lâché hors de la table : ramené au coin le plus proche.
	e.MoveSeat("u-alice", -100, tableH+100)
	if p.AX != feltInset || p.AY != tableH-feltInset {
		t.Fatalf("coin bas gauche attendu, obtenu (%.1f, %.1f)", p.AX, p.AY)
	}
	// La place survit à une absence (onglet fermé puis rouvert).
	e.removePlayer("u-alice")
	q := e.ensurePlayer("u-alice", "alice", tableW, tableH)
	if q.AX != feltInset || q.AY != tableH-feltInset {
		t.Fatalf("la place choisie doit être retrouvée au retour, obtenu (%.1f, %.1f)", q.AX, q.AY)
	}
	// Valeurs absurdes et joueur parti : refusés.
	if e.MoveSeat("u-alice", math.NaN(), 10) {
		t.Fatal("une position NaN doit être refusée")
	}
	e.Abandon("u-alice", AbandonToTable)
	if e.MoveSeat("u-alice", 600, feltInset) {
		t.Fatal("un joueur parti ne déplace plus son avatar")
	}
	if e.MoveSeat("u-inconnu", 600, feltInset) {
		t.Fatal("un joueur absent de la table ne déplace rien")
	}
}

func TestAvatarsKeepTheirDistance(t *testing.T) {
	e := newEngine()
	e.ensurePlayer("u-alice", "alice", tableW, tableH) // siège sud (720, 760)
	e.ensurePlayer("u-bob", "bob", tableW, tableH)
	alice := e.players["u-alice"]
	// Bob se pose presque sur Alice : il glisse jusqu'à la place libre.
	if !e.MoveSeat("u-bob", alice.AX+20, alice.AY) {
		t.Fatal("le déplacement aurait dû aboutir à une place libre")
	}
	bob := e.players["u-bob"]
	if d := math.Hypot(bob.AX-alice.AX, bob.AY-alice.AY); d < seatGap-0.01 || d > seatGap+2.5 {
		t.Fatalf("bob devrait être juste à %.0f de alice, il est à %.1f", seatGap, d)
	}
	if bob.AX <= alice.AX {
		t.Fatalf("bob, lâché à droite de alice, devrait glisser à droite (x=%.1f)", bob.AX)
	}
	// Un nouvel arrivant dont le siège par défaut (nord) est occupé est décalé.
	e.MoveSeat("u-alice", tableW/2, 0) // alice s'installe au nord
	e.ensurePlayer("u-carl", "carl", tableW, tableH)
	e.ensurePlayer("u-dana", "dana", tableW, tableH) // siège 3 = nord, pris par alice
	dana, al := e.players["u-dana"], e.players["u-alice"]
	if d := math.Hypot(dana.AX-al.AX, dana.AY-al.AY); d < seatGap-0.01 {
		t.Fatalf("dana ne doit pas chevaucher alice (distance %.1f)", d)
	}
}

func TestShuffleSabotKeepsSameCardsAndLeavesTableAndHandsAlone(t *testing.T) {
	e := newEngineWithCards(t, 52)
	e.ensurePlayer("u-alice", "alice", tableW, tableH)
	inHand, _ := e.DrawSabot(Transfer{Target: TargetHand, OwnerID: "u-alice"})
	onTable, _ := e.DrawSabot(Transfer{Target: TargetTable, X: 300, Y: 300})
	before := append([]string(nil), e.sabot...)

	if n := e.ShuffleSabot(); n != 50 {
		t.Fatalf("ShuffleSabot devrait annoncer 50 cartes, a annoncé %d", n)
	}
	if len(e.sabot) != len(before) {
		t.Fatalf("le mélange ne doit ni ajouter ni retirer de carte: %d -> %d", len(before), len(e.sabot))
	}
	want := map[string]int{}
	for _, id := range before {
		want[id]++
	}
	for _, id := range e.sabot {
		want[id]--
	}
	for id, d := range want {
		if d != 0 {
			t.Fatalf("la carte %s n'a pas survécu intacte au mélange (écart %d)", id, d)
		}
	}
	// Le mélange porte sur le paquet, pas sur la table : rien n'est ramassé.
	if c := e.findCard(inHand); c.Zone != ZoneHand || c.Owner != "u-alice" {
		t.Fatalf("la carte en main ne doit pas bouger, zone=%v owner=%q", c.Zone, c.Owner)
	}
	if c := e.findCard(onTable); c.Zone != ZoneTable {
		t.Fatalf("la carte du tapis ne doit pas bouger, zone=%v", c.Zone)
	}
	// 50! ordres possibles : retomber sur l'ordre initial signalerait un
	// mélange inopérant, pas un hasard.
	same := true
	for i := range before {
		if before[i] != e.sabot[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("l'ordre du sabot devrait avoir changé")
	}
}

func TestShuffleSabotIsANoOpBelowTwoCards(t *testing.T) {
	e := newEngineWithCards(t, 1)
	if n := e.ShuffleSabot(); n != 0 {
		t.Fatalf("un sabot d'une carte n'a rien à mélanger, annoncé %d", n)
	}
	e.DrawSabot(Transfer{Target: TargetTable, X: 300, Y: 300})
	if n := e.ShuffleSabot(); n != 0 {
		t.Fatalf("un sabot vide n'a rien à mélanger, annoncé %d", n)
	}
}

func TestTransferManyToSabotBottomKeepsPileOrderUnderTheShoe(t *testing.T) {
	e := newEngineWithCards(t, 6)
	var ids []string
	for i := 0; i < 3; i++ {
		id, _ := e.DrawSabot(Transfer{Target: TargetTable, X: float64(100 + 10*i), Y: 200})
		ids = append(ids, id)
	}
	sabotBefore := len(e.sabot)

	// Payload volontairement désordonné : c'est l'ordre Z qui doit primer.
	changed, _ := e.TransferManyToSabotBottom([]string{ids[2], ids[0], ids[1]})
	if !changed {
		t.Fatal("le transfert aurait dû modifier l'état")
	}
	if len(e.sabot) != sabotBefore+3 {
		t.Fatalf("le sabot devrait regagner 3 cartes: %d -> %d", sabotBefore, len(e.sabot))
	}
	// Les trois cartes occupent le fond, dans leur ordre Z croissant.
	for i, want := range ids {
		if e.sabot[i] != want {
			t.Fatalf("fond du sabot position %d: attendu %q, trouvé %q", i, want, e.sabot[i])
		}
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
	if c := e.findCard(inHand); !c.FaceUp {
		t.Fatal("la carte en main (distribuée face cachée) devrait être retournée face visible")
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
	e.findCard(top).FaceUp = true // doit le rester : aucun retournement automatique
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
	if c := e.findCard(top); !c.FaceUp {
		t.Fatal("une carte face visible remise au sabot doit rester face visible")
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
