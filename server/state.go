package main

import (
	"math"
	"sort"
	"sync"
)

// ---- Zones ----------------------------------------------------------------

// Une carte se trouve toujours dans l'une de ces zones. Aucune règle de jeu
// n'est attachée à ces zones : elles décrivent seulement l'emplacement physique
// d'une carte, comme sur une vraie table.
type Zone string

const (
	ZoneSabot Zone = "sabot" // dans le sabot (empilé)
	ZoneTable Zone = "table" // sur le tapis (publique, manipulable)
	ZoneHand  Zone = "hand"  // dans la main d'un joueur (privée)
)

func (z Zone) public() bool { return z == ZoneSabot || z == ZoneTable }

// ---- Card -----------------------------------------------------------------

// Card est l'unique représentation d'une carte, maître absolu côté serveur.
// Le client n'en est que le miroir. Aucun attribut métier (valeur, atout...)
// n'est présent : le système ignore les règles de jeu.
type Card struct {
	ID     string `json:"id"`     // identifiant stable (ex. "c-12")
	FaceID string `json:"faceId"` // symbole du sprite ("1_club", "king_spade", "black_joker", "back")
	Zone   Zone   `json:"zone"`   // sabot | table | hand
	Owner  string `json:"owner"`  // userID propriétaire quand Zone==ZoneHand
	// Position sur le tapis (px relatifs à la zone table). Zone==ZoneTable uniquement.
	X float64 `json:"x,omitempty"`
	Y float64 `json:"y,omitempty"`
	// Ordre de superposition (Z). Plus grand = devant. Zone==ZoneTable uniquement.
	Z      int     `json:"z,omitempty"`
	Rotate float64 `json:"rotate,omitempty"` // degrés
	FaceUp bool    `json:"faceUp"`           // recto visible (sinon dos)
}

// ---- Player ---------------------------------------------------------------

// Player décrit un participant connecté. Le serveur reste l'unique source de
// vérité de cette liste.
type Player struct {
	UserID string `json:"userId"`
	Name   string `json:"name"`
	// Position de l'avatar sur le tapis (px, relatifs à la zone table).
	AX float64 `json:"ax"`
	AY float64 `json:"ay"`
	// Nombre de cartes dans sa main (compte seul, jamais les cartes
	// elles-mêmes : la main reste privée). Recalculé à chaque snapshot
	// public, cf. snapshotPublic — ne pas alimenter ce champ ailleurs.
	HandCount int `json:"handCount"`
	// Abandoned marque un joueur qui a QUITTÉ la partie, par opposition à une
	// simple absence. Sa chaise reste affichée, grisée : les autres joueurs
	// voient que la place est vide pour de bon. Un joueur absent, lui, est
	// retiré de la liste et son avatar disparaît le temps de son retour.
	Abandoned bool `json:"abandoned"`
}

// ---- Engine ---------------------------------------------------------------

// engine détient l'état autoritaire et sérialise toutes les mutations.
type engine struct {
	mu      sync.Mutex
	cards   []Card             // toutes les cartes (maître)
	sabot   []string           // IDs empilés dans le sabot (fond -> sommet)
	players map[string]*Player // userID -> Player (connectés)
	// seats retient le siège attribué à chaque joueur, y compris après son
	// départ. Cette mémoire est ce qui permet de retrouver sa chaise en
	// revenant : voir ensurePlayer.
	seats map[string]int // userID -> indice de siège
	zTop  int            // compteur d'ordre Z (croissant = devant)
}

func newEngine() *engine {
	return &engine{players: map[string]*Player{}, seats: map[string]int{}}
}

// ---- Player management ----------------------------------------------------

// ensurePlayer ajoute le joueur s'il est nouveau et renvoie sa fiche. La
// position de l'avatar est calculée autour de la table (répartie angulairement).
//
// Le siège est ATTRIBUÉ UNE FOIS PAR COMPTE et conservé dans e.seats, y compris
// pendant que le joueur est absent. Deux défauts successifs l'imposent :
//
//  1. avec le nombre de joueurs connectés (len(e.players)) comme index, un
//     joueur qui revenait reprenait le siège d'un joueur resté présent, et les
//     deux avatars se superposaient exactement.
//  2. avec un compteur d'arrivée qui ne faisait que croître, le défaut inverse
//     apparaissait : rafraîchir sa page change de chaise. Fermer l'onglet ferme
//     la WebSocket, la dernière connexion du compte disparaît et le Player est
//     retiré (cf. main.go) ; en revenant, le joueur recevait le siège suivant,
//     et faisait le tour de la table en six rafraîchissements.
//
// Retenir le siège par userID règle les deux : l'index ne dépend plus de qui est
// connecté à cet instant. La mémoire vit en RAM, comme le reste de l'état — un
// redémarrage du serveur rebat les places, ce qui est cohérent avec l'absence
// assumée de persistance.
func (e *engine) ensurePlayer(userID, name string, tableW, tableH float64) *Player {
	if p, ok := e.players[userID]; ok {
		p.Name = name
		// Un joueur qui avait abandonné et qui revient reprend place pour de
		// bon : sa chaise cesse d'être grisée. Sa main, elle, ne revient pas —
		// ses cartes sont reparties sur le tapis ou au fond du sabot.
		p.Abandoned = false
		return p
	}
	seat, known := e.seats[userID]
	if !known {
		seat = e.freeSeat()
		e.seats[userID] = seat
	}
	p := &Player{UserID: userID, Name: name}
	e.layoutAvatar(p, seat, tableW, tableH)
	e.players[userID] = p
	return p
}

// freeSeat renvoie le plus petit indice de siège qu'aucun compte connu ne s'est
// déjà vu attribuer. On balaie les sièges attribués, et non les seuls joueurs
// connectés : sinon un joueur absent se verrait voler sa chaise pendant qu'il
// recharge sa page, et la retrouverait occupée en revenant.
func (e *engine) freeSeat() int {
	taken := make(map[int]bool, len(e.seats))
	for _, s := range e.seats {
		taken[s] = true
	}
	for i := 0; ; i++ {
		if !taken[i] {
			return i
		}
	}
}

// layoutAvatar place un avatar autour de la table selon son rang d'arrivée.
func (e *engine) layoutAvatar(p *Player, index int, w, h float64) {
	if w <= 0 {
		w = 800
	}
	if h <= 0 {
		h = 500
	}
	const seats = 6
	// Les avatars ("chaises") reposent majoritairement sur le pourtour bois
	// de la table (§7), et ne mordent que légèrement sur le feutre vert, pour
	// laisser un maximum d'espace de jeu. L'amplitude reste bornée par la
	// hauteur totale de la table (w,h) : un siège dont le centre + la moitié
	// de sa hauteur dépasserait h sortirait complètement de la zone
	// défilable (perdu, pas seulement hors champ).
	cx, cy := w/2, h/2
	rx, ry := w*0.515, h*0.431
	// Angle décalé pour que le siège 0 soit en bas (sud), face à la table.
	a := float64(index%seats)*(2*math.Pi/seats) + math.Pi/2
	p.AX = cx + rx*math.Cos(a)
	p.AY = cy + ry*math.Sin(a)
}

// removePlayer retire le joueur de la liste des connectés. Son siège reste
// réservé dans e.seats : c'est précisément ce qui lui rend sa chaise au retour
// (cf. ensurePlayer). Ne pas y ajouter de delete sur e.seats.
//
// Un joueur ayant abandonné fait exception : il reste dans e.players pour que
// sa chaise continue d'apparaître, grisée. Sans cela, la fermeture de sa
// WebSocket — qui suit immédiatement son abandon — effacerait la trace de son
// départ, et les autres joueurs ne verraient qu'un avatar disparu, exactement
// comme pour une absence passagère.
func (e *engine) removePlayer(userID string) {
	if p, ok := e.players[userID]; ok && p.Abandoned {
		return
	}
	delete(e.players, userID)
}

// ---- Abandon de partie ----------------------------------------------------

// AbandonPolicy décide du sort de la main d'un joueur qui quitte la partie.
// Le choix appartient au partant, qui le fait au moment de quitter ; ce n'est
// pas un réglage de table. Aucune des deux options ne révèle ses cartes : un
// joueur qui part ne doit pas donner d'information à ceux qui restent.
type AbandonPolicy string

const (
	// AbandonToTable étale la main, face cachée, devant la chaise du partant.
	// Les cartes restent en jeu et visibles de tous, comme un joueur qui pose
	// son jeu et se lève.
	AbandonToTable AbandonPolicy = "table"
	// AbandonToSabot remet la main au FOND du sabot. Les cartes retournent au
	// jeu sans que personne ne sache lesquelles, et ne ressortiront qu'une fois
	// tout le reste du sabot épuisé.
	AbandonToSabot AbandonPolicy = "sabot"
)

// normalizeAbandonPolicy retombe sur le dépôt sur tapis pour toute valeur
// inconnue : c'est la moins destructrice des deux (les cartes restent
// atteignables, là où le fond du sabot les enfouit).
func normalizeAbandonPolicy(raw string) AbandonPolicy {
	if AbandonPolicy(raw) == AbandonToSabot {
		return AbandonToSabot
	}
	return AbandonToTable
}

// Abandon fait quitter la partie au joueur : sa main part selon le choix qu'il
// vient de faire, et sa chaise reste affichée en grisé. À la différence d'une
// déconnexion, l'opération est irréversible — revenir ne rend pas les cartes.
// Renvoie false si le joueur n'est pas à table.
func (e *engine) Abandon(userID string, policy AbandonPolicy) bool {
	p, ok := e.players[userID]
	if !ok {
		return false
	}
	var hand []*Card
	for i := range e.cards {
		if e.cards[i].Zone == ZoneHand && e.cards[i].Owner == userID {
			hand = append(hand, &e.cards[i])
		}
	}
	switch policy {
	case AbandonToSabot:
		e.sendToSabotBottom(hand)
	default:
		e.abandonToTable(hand, p)
	}
	p.Abandoned = true
	return true
}

// sendToSabotBottom renvoie des cartes au FOND du sabot. e.sabot est ordonné du
// fond (index 0) vers le sommet : on préfixe donc, au lieu d'ajouter comme le
// fait un dépôt ordinaire sur le sabot.
//
// Le lot est inséré d'un bloc, et non carte par carte : préfixer en boucle
// inverserait l'ordre du paquet. Servi aussi bien par l'abandon que par
// l'action « Au fond du sabot » de la sélection multiple.
func (e *engine) sendToSabotBottom(cards []*Card) {
	if len(cards) == 0 {
		return
	}
	ids := make([]string, 0, len(cards))
	for _, c := range cards {
		c.Zone = ZoneSabot
		c.Owner = ""
		c.FaceUp = false
		c.X, c.Y, c.Z, c.Rotate = 0, 0, 0, 0
		ids = append(ids, c.ID)
	}
	e.sabot = append(ids, e.sabot...)
}

// TransferManyToSabotBottom enfouit un lot de cartes de table sous le sabot.
// Symétrique de TransferMany vers le sabot, qui les empile au sommet : ici on
// veut qu'elles ne ressortent qu'en dernier. Les cartes sont traitées par Z
// croissant pour que l'ordre d'une pile enfouie soit préservé. Les IDs inconnus,
// dupliqués ou déjà au sabot sont ignorés, comme ailleurs. Renvoie l'ensemble
// des mains impactées, à notifier en privé.
func (e *engine) TransferManyToSabotBottom(ids []string) (changed bool, fromHands map[string]bool) {
	fromHands = map[string]bool{}
	seen := map[string]bool{}
	var picked []*Card
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		c := e.findCard(id)
		if c == nil || c.Zone == ZoneSabot {
			continue
		}
		if c.Zone == ZoneHand {
			fromHands[c.Owner] = true
		}
		picked = append(picked, c)
	}
	if len(picked) == 0 {
		return false, fromHands
	}
	sort.SliceStable(picked, func(i, j int) bool { return picked[i].Z < picked[j].Z })
	e.sendToSabotBottom(picked)
	return true, fromHands
}

// abandonToTable étale la main face cachée devant la chaise du partant, pour
// qu'on voie d'où elle vient. La rangée est centrée sur l'avatar puis ramenée
// dans le feutre : les sièges sont calculés sur le pourtour bois, donc en
// dehors de la zone où une carte a le droit d'être posée.
func (e *engine) abandonToTable(cards []*Card, p *Player) {
	n := len(cards)
	if n == 0 {
		return
	}
	spacing := abandonSpacing
	if n > 1 {
		if fit := (maxCardX - minCardX) / float64(n-1); fit < spacing {
			spacing = fit
		}
	}
	x0 := p.AX - (float64(n-1)*spacing)/2 - cardW/2
	y := clampF(p.AY-cardH/2, minCardY, maxCardY)
	for i, c := range cards {
		c.Zone = ZoneTable
		c.Owner = ""
		c.FaceUp = false // un abandon ne révèle jamais le jeu du partant
		c.Rotate = 0
		c.X = clampF(x0+float64(i)*spacing, minCardX, maxCardX)
		c.Y = y
		c.Z = e.nextZ()
	}
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ---- State init (sabot) ---------------------------------------------------

// LoadDeck charge un sabot de cartes (issu de la config du menu init). Toute
// ancienne table est remplacée ; les joueurs sont conservés. Conforme au §13 :
// aucun mélange/distribution "intelligent", les cartes sont simplement placées
// dans le sabot dans l'ordre reçu.
func (e *engine) LoadDeck(cards []Card) {
	// Nouvelle partie : les chaises grisées des joueurs partis disparaissent.
	// Elles marquaient un abandon dans la partie précédente, qui n'a plus cours.
	for id, p := range e.players {
		if p.Abandoned {
			delete(e.players, id)
		}
	}
	e.cards = make([]Card, len(cards))
	e.sabot = make([]string, 0, len(cards))
	for i, c := range cards {
		c.Zone = ZoneSabot
		c.Owner = ""
		c.FaceUp = false // sabot = face cachée (comme une vraie shoe)
		c.X, c.Y = 0, 0
		c.Z = 0
		c.Rotate = 0
		e.cards[i] = c
		e.sabot = append(e.sabot, c.ID)
	}
	e.zTop = 0
}

// Initialized indique si un sabot a été chargé.
func (e *engine) Initialized() bool { return len(e.cards) > 0 }

// ---- Helpers (sous e.mu verrouillé) --------------------------------------

// findCard retourne un pointeur vers la carte d'ID donné, ou nil.
func (e *engine) findCard(id string) *Card {
	for i := range e.cards {
		if e.cards[i].ID == id {
			return &e.cards[i]
		}
	}
	return nil
}

// nextZ incrémente et renvoie le prochain ordre Z (au premier plan).
func (e *engine) nextZ() int {
	e.zTop++
	return e.zTop
}

// ---- Mutations atomiques (appelées sous e.mu verrouillé) ------------------

// Flip retourne la carte (recto/verso). Autorisé sur table et en main.
// handOwner est non vide si la carte retournée se trouve dans une main
// privée : le serveur doit alors notifier ce joueur directement, car cette
// mutation n'apparaît jamais dans l'état public (snapshotPublic exclut les
// mains privées).
func (e *engine) Flip(cardID string) (ok bool, handOwner string) {
	c := e.findCard(cardID)
	if c == nil || c.Zone == ZoneSabot {
		return false, ""
	}
	c.FaceUp = !c.FaceUp
	if c.Zone == ZoneHand {
		return true, c.Owner
	}
	return true, ""
}

// BringToFront place une carte de table au premier plan (Z maximum).
func (e *engine) BringToFront(cardID string) bool {
	c := e.findCard(cardID)
	if c == nil || c.Zone != ZoneTable {
		return false
	}
	c.Z = e.nextZ()
	return true
}

// Rotate applique un angle à une carte de table.
func (e *engine) Rotate(cardID string, deg float64) bool {
	c := e.findCard(cardID)
	if c == nil || c.Zone != ZoneTable {
		return false
	}
	c.Rotate = deg
	return true
}

// Move repositionne une carte de table (drag terminé) et la ramène au premier plan.
func (e *engine) Move(cardID string, x, y float64) bool {
	c := e.findCard(cardID)
	if c == nil || c.Zone != ZoneTable {
		return false
	}
	c.X, c.Y = x, y
	c.Z = e.nextZ()
	return true
}

// ---- Mutations par lot (sélection multiple) --------------------------------

// CardMove décrit la position cible d'une carte dans un déplacement groupé.
type CardMove struct {
	CardID string
	X, Y   float64
}

// MoveMany repositionne un lot de cartes de table (drag groupé terminé) et
// ramène le groupe au premier plan en préservant l'ordre Z relatif des cartes
// entre elles : tri par Z courant, puis attribution de Z consécutifs. Sans ce
// tri, une pile déplacée en bloc verrait ses cartes réordonnées selon l'ordre
// arbitraire du payload client. Les IDs inconnus, dupliqués ou hors table sont
// ignorés silencieusement (même tolérance que les mutations mono-carte).
// Renvoie true si au moins une carte a bougé.
func (e *engine) MoveMany(items []CardMove) bool {
	type sel struct {
		c    *Card
		x, y float64
	}
	var picked []sel
	seen := map[string]bool{}
	for _, it := range items {
		if seen[it.CardID] {
			continue
		}
		seen[it.CardID] = true
		c := e.findCard(it.CardID)
		if c == nil || c.Zone != ZoneTable {
			continue
		}
		picked = append(picked, sel{c, it.X, it.Y})
	}
	if len(picked) == 0 {
		return false
	}
	sort.SliceStable(picked, func(i, j int) bool { return picked[i].c.Z < picked[j].c.Z })
	for _, s := range picked {
		s.c.X, s.c.Y = s.x, s.y
		s.c.Z = e.nextZ()
	}
	return true
}

// FlipMany retourne un lot de cartes en une seule mutation (table ou main,
// jamais le sabot — même règle que Flip). Les IDs dupliqués ne sont retournés
// qu'une fois (un double flip serait un no-op trompeur). Renvoie si quelque
// chose a changé et l'ensemble des propriétaires de mains impactés, chacun
// devant recevoir une notification privée (cf. Flip).
func (e *engine) FlipMany(ids []string) (changed bool, handOwners map[string]bool) {
	handOwners = map[string]bool{}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		c := e.findCard(id)
		if c == nil || c.Zone == ZoneSabot {
			continue
		}
		c.FaceUp = !c.FaceUp
		changed = true
		if c.Zone == ZoneHand {
			handOwners[c.Owner] = true
		}
	}
	return changed, handOwners
}

// BatchTransferResult agrège l'issue d'un transfert groupé à diffuser.
type BatchTransferResult struct {
	PublicChanged  bool
	HandOwners     map[string]bool // mains ayant gagné des cartes (notif ciblée)
	FromHandOwners map[string]bool // mains ayant perdu des cartes (notif ciblée)
}

// TransferMany transfère un lot de cartes vers une même cible (sabot, avatar
// ou main) en une seule mutation. Un lot vers la TABLE est refusé : les
// positions sont individuelles, c'est le rôle de MoveMany. Les cartes sont
// traitées par Z croissant : remettre une pile dans le sabot conserve ainsi
// son ordre (la carte du dessus de la pile finit au sommet du sabot), quel
// que soit l'ordre du payload client. Les cartes encore au sabot sont
// exclues : leur retrait de e.sabot n'est géré que par DrawSabot.
func (e *engine) TransferMany(ids []string, target DropTarget, ownerID string) BatchTransferResult {
	res := BatchTransferResult{HandOwners: map[string]bool{}, FromHandOwners: map[string]bool{}}
	if target == TargetTable {
		return res
	}
	seen := map[string]bool{}
	var picked []*Card
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		c := e.findCard(id)
		if c == nil || c.Zone == ZoneSabot {
			continue
		}
		picked = append(picked, c)
	}
	sort.SliceStable(picked, func(i, j int) bool { return picked[i].Z < picked[j].Z })
	for _, c := range picked {
		r := e.applyTransfer(c, c.Zone, Transfer{CardID: c.ID, Target: target, OwnerID: ownerID}, false)
		res.PublicChanged = res.PublicChanged || r.PublicChanged
		if r.HandOwner != "" {
			res.HandOwners[r.HandOwner] = true
		}
		if r.FromHandOwner != "" {
			res.FromHandOwners[r.FromHandOwner] = true
		}
	}
	return res
}

// ---- Transferts entre zones ----------------------------------------------

// DropTarget décrit la cible d'un drag-and-drop.
type DropTarget string

const (
	TargetTable  DropTarget = "table"
	TargetSabot  DropTarget = "sabot"
	TargetAvatar DropTarget = "avatar"
	TargetHand   DropTarget = "hand"
)

// Transfer décrit un déplacement de carte entre zones.
type Transfer struct {
	CardID  string     `json:"cardId"`
	Target  DropTarget `json:"target"`
	X       float64    `json:"x,omitempty"`
	Y       float64    `json:"y,omitempty"`
	OwnerID string     `json:"ownerId,omitempty"` // cible si avatar/hand
}

// TransferResult décrit l'issue d'une mutation à diffuser.
type TransferResult struct {
	PublicChanged bool   // l'état public (table/sabot) a changé -> broadcast
	HandOwner     string // une carte est entrée dans la main de ce joueur (notif ciblée)
	FromHandOwner string // une carte est sortie de la main de ce joueur (notif ciblée)
}

// applyTransfer réalise le transfert d'une carte déjà identifiée vers une cible.
// fromZone = zone de la carte AVANT l'opération. dealt indique une véritable
// distribution (tirage depuis le sabot vers une main, cf. DrawSabot) : seul ce
// cas révèle la carte. Un dépôt sur la TABLE ne révèle jamais la carte, qu'il
// s'agisse d'un simple déplacement ou d'un tirage direct sabot→tapis (une
// carte n'est révélée que lorsqu'elle est donnée à un joueur, jamais posée
// face visible sur le tapis automatiquement). Un simple déplacement (drag
// main→tapis, tapis→main d'un autre joueur, etc. via TransferCard) ne doit
// JAMAIS changer l'état face d'une carte : un joueur peut avoir choisi de
// retourner une carte de sa main avant de la poser, ce choix doit être
// respecté.
func (e *engine) applyTransfer(c *Card, fromZone Zone, t Transfer, dealt bool) TransferResult {
	// Propriétaire AVANT mutation : non vide seulement si la carte venait
	// d'une main (fromZone == ZoneHand). Permet de notifier ce joueur que sa
	// main a perdu une carte, quelle que soit la destination (sinon la carte
	// restait affichée dans sa main jusqu'au prochain rafraîchissement).
	prevHandOwner := ""
	if fromZone == ZoneHand {
		prevHandOwner = c.Owner
	}
	switch t.Target {
	case TargetTable:
		// hand→table, table→table ou sabot→table : pose à la position de
		// relâchement (§6), jamais révélée automatiquement.
		c.Zone = ZoneTable
		c.Owner = ""
		c.X, c.Y = t.X, t.Y
		c.Z = e.nextZ()
		return TransferResult{PublicChanged: true, FromHandOwner: prevHandOwner}

	case TargetSabot:
		// table→sabot : remise dans la shoe, toujours face cachée, au sommet.
		c.Zone = ZoneSabot
		c.Owner = ""
		c.FaceUp = false
		c.X, c.Y = 0, 0
		c.Rotate = 0
		c.Z = 0
		e.sabot = append(e.sabot, c.ID)
		return TransferResult{PublicChanged: true, FromHandOwner: prevHandOwner}

	case TargetAvatar, TargetHand:
		// table→avatar / hand→hand / hand→avatar : carte vers la main privée.
		if t.OwnerID == "" {
			return TransferResult{}
		}
		// On ne donne pas de carte à un joueur parti : sa main n'a plus de
		// destinataire, et la carte disparaîtrait du tapis sans que personne
		// puisse la reprendre. Le client lui retire déjà sa qualité de cible,
		// mais c'est le serveur qui fait autorité.
		if dst, ok := e.players[t.OwnerID]; ok && dst.Abandoned {
			return TransferResult{}
		}
		c.Zone = ZoneHand
		c.Owner = t.OwnerID
		c.X, c.Y = 0, 0
		c.Rotate = 0
		c.Z = 0
		if dealt {
			c.FaceUp = true // distribution depuis le sabot : visible par son propriétaire
		}
		return TransferResult{
			PublicChanged: fromZone.public(), // si elle venait de la table/sabot, le public change
			HandOwner:     t.OwnerID,
			FromHandOwner: prevHandOwner,
		}
	}
	return TransferResult{}
}

// TransferCard applique un transfert sur une carte identifiée par son ID. Ce
// n'est jamais une distribution (dealt=false) : un simple drag ne change pas
// l'état face de la carte, quelle que soit la zone source ou destination.
func (e *engine) TransferCard(t Transfer) TransferResult {
	c := e.findCard(t.CardID)
	if c == nil {
		return TransferResult{}
	}
	fromZone := c.Zone
	return e.applyTransfer(c, fromZone, t, false)
}

// DrawSabot tire la carte au sommet du sabot vers une cible (drag du sabot, §6).
// Retourne l'ID tiré et le résultat de diffusion. Aucune règle : on tire
// simplement le dessus de la pile.
func (e *engine) DrawSabot(t Transfer) (string, TransferResult) {
	n := len(e.sabot)
	if n == 0 {
		return "", TransferResult{}
	}
	id := e.sabot[n-1]
	e.sabot = e.sabot[:n-1]
	c := e.findCard(id)
	if c == nil {
		return "", TransferResult{}
	}
	// Une carte tirée du sabot devient publique (changement public) puis suit
	// la cible du drop. C'est une véritable distribution (dealt=true) : elle
	// est révélée dans sa nouvelle zone.
	res := e.applyTransfer(c, ZoneSabot, t, true)
	return id, res
}

// ---- Snapshots (sérialisation) -------------------------------------------

// publicState est la vue diffusée à TOUS les clients : sabot (décompte), cartes
// de table (publiques), joueurs connectés. Les mains privées en sont exclues.
type publicState struct {
	Type        string   `json:"type"` // toujours "state"
	SabotCount  int      `json:"sabotCount"`
	Table       []Card   `json:"table"`
	Players     []Player `json:"players"`
	Initialized bool     `json:"initialized"`
}

// handPayload est la vue privée envoyée au seul propriétaire d'une main.
type handPayload struct {
	Cards []Card `json:"cards"`
}

// snapshotPublic construit l'état public complet.
func (e *engine) snapshotPublic() publicState {
	out := publicState{Type: "state", SabotCount: len(e.sabot), Initialized: e.Initialized()}
	handCounts := make(map[string]int, len(e.players))
	for _, c := range e.cards {
		if c.Zone == ZoneTable {
			out.Table = append(out.Table, c)
		} else if c.Zone == ZoneHand {
			handCounts[c.Owner]++
		}
	}
	// Tri stable par Z croissant pour un rendu correct de la superposition.
	sort.SliceStable(out.Table, func(i, j int) bool {
		return out.Table[i].Z < out.Table[j].Z
	})
	for _, p := range e.players {
		pc := *p
		pc.HandCount = handCounts[p.UserID]
		out.Players = append(out.Players, pc)
	}
	// Ordre stable des joueurs (par userID) pour un diff propre.
	sort.SliceStable(out.Players, func(i, j int) bool {
		return out.Players[i].UserID < out.Players[j].UserID
	})
	return out
}

// snapshotHand construit la main privée d'un joueur.
func (e *engine) snapshotHand(userID string) handPayload {
	var h handPayload
	for _, c := range e.cards {
		if c.Zone == ZoneHand && c.Owner == userID {
			h.Cards = append(h.Cards, c)
		}
	}
	return h
}
