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
	UserID string  `json:"userId"`
	Name   string  `json:"name"`
	// Position de l'avatar sur le tapis (px, relatifs à la zone table).
	AX float64 `json:"ax"`
	AY float64 `json:"ay"`
}

// ---- Engine ---------------------------------------------------------------

// engine détient l'état autoritaire et sérialise toutes les mutations.
type engine struct {
	mu        sync.Mutex
	cards     []Card              // toutes les cartes (maître)
	sabot     []string            // IDs empilés dans le sabot (fond -> sommet)
	players   map[string]*Player  // userID -> Player (connectés)
	nextSeat  int                 // compteur d'arrivée, jamais réutilisé (voir ensurePlayer)
	zTop      int                 // compteur d'ordre Z (croissant = devant)
}

func newEngine() *engine {
	return &engine{players: map[string]*Player{}}
}

// ---- Player management ----------------------------------------------------

// ensurePlayer ajoute le joueur s'il est nouveau et renvoie sa fiche. La
// position de l'avatar est calculée autour de la table (répartie angulairement).
//
// Le siège utilise un compteur d'arrivée qui ne fait que croître (nextSeat),
// jamais le nombre de joueurs actuellement connectés (len(e.players)) : sinon
// un joueur qui se déconnecte puis se reconnecte reprend le même index qu'un
// autre joueur toujours présent, et les deux avatars se superposent
// exactement à l'écran (l'un masque totalement l'autre).
func (e *engine) ensurePlayer(userID, name string, tableW, tableH float64) *Player {
	if p, ok := e.players[userID]; ok {
		p.Name = name
		return p
	}
	p := &Player{UserID: userID, Name: name}
	e.layoutAvatar(p, e.nextSeat, tableW, tableH)
	e.nextSeat++
	e.players[userID] = p
	return p
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

func (e *engine) removePlayer(userID string) {
	delete(e.players, userID)
}

// ---- State init (sabot) ---------------------------------------------------

// LoadDeck charge un sabot de cartes (issu de la config du menu init). Toute
// ancienne table est remplacée ; les joueurs sont conservés. Conforme au §13 :
// aucun mélange/distribution "intelligent", les cartes sont simplement placées
// dans le sabot dans l'ordre reçu.
func (e *engine) LoadDeck(cards []Card) {
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
	for _, c := range e.cards {
		if c.Zone == ZoneTable {
			out.Table = append(out.Table, c)
		}
	}
	// Tri stable par Z croissant pour un rendu correct de la superposition.
	sort.SliceStable(out.Table, func(i, j int) bool {
		return out.Table[i].Z < out.Table[j].Z
	})
	for _, p := range e.players {
		out.Players = append(out.Players, *p)
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
