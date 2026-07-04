package main

import (
	"encoding/json"
	"log"
	"strings"
	"time"
)

// Dimensions de design de la zone table (px). Sert uniquement à positionner les
// avatars autour de la table côté serveur. Le client les utilise comme taille
// logique du tapis.
const (
	tableW = 1000.0
	tableH = 640.0
)

// ---- Payloads clients -----------------------------------------------------

type payloadInit struct {
	Config DeckConfig `json:"config"`
	Shuffle bool      `json:"shuffle"`
}

type payloadCardOp struct {
	CardID  string  `json:"cardId"`
	X       float64 `json:"x,omitempty"`
	Y       float64 `json:"y,omitempty"`
	Rotate  float64 `json:"rotate,omitempty"`
}

type payloadTransfer struct {
	CardID  string  `json:"cardId"`
	Target  string  `json:"target"`
	X       float64 `json:"x,omitempty"`
	Y       float64 `json:"y,omitempty"`
	OwnerID string  `json:"ownerId,omitempty"`
}

type payloadDrag struct {
	CardID string  `json:"cardId"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
}

type payloadChat struct {
	Text string `json:"text"`
}

// ---- Diffusion ------------------------------------------------------------

// broadcastMsg sérialise un Message et le diffuse à toute la room (sauf except).
func (app *application) broadcastMsg(room string, msg Message, except *client) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	app.hub.broadcast(room, data, except)
}

// sendMsg envoie un Message à un client précis (ex. main privée).
func (app *application) sendMsg(c *client, msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	app.hub.sendTo(c, data)
}

// broadcastState reconstruit et diffuse l'état public complet à toute la room.
// Utilisé après toute mutation publique pour garantir la cohérence stricte
// entre tous les clients (§12).
func (app *application) broadcastState(room string) {
	app.engine.mu.Lock()
	st := app.engine.snapshotPublic()
	app.engine.mu.Unlock()
	data, err := json.Marshal(Message{Type: "state", Payload: mustJSON(st)})
	if err != nil {
		return
	}
	app.hub.broadcast(room, data, nil)
}

// sendHand envoie la main privée d'un joueur à lui seul (§6/§7 : cartes
// uniquement visibles par le propriétaire).
func (app *application) sendHand(room, userID string) {
	app.engine.mu.Lock()
	h := app.engine.snapshotHand(userID)
	app.engine.mu.Unlock()
	for _, c := range app.hub.clients(room) {
		if c.userID == userID {
			app.sendMsg(c, Message{Type: "hand", Payload: mustJSON(h)})
		}
	}
}

// sendPresence diffuse la liste des joueurs connectés (join/leave). On réutilise
// l'état public pour rester cohérent.
func (app *application) sendPresence(room string) {
	app.broadcastState(room)
}

// mustJSON sérialise v ou renvoie null. Utilisé pour des payloads internes où
// un échec de sérialisation n'a pas de sens (struct simples).
func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("null")
	}
	return json.RawMessage(b)
}

// ---- Dispatch des messages clients ---------------------------------------

// handleClientMsg traite un message reçu d'un client authentifié. Toute
// mutation passe par le moteur sous mutex : le serveur reste autoritaire et
// validera uniquement la cohérence technique (pas de règle de jeu, §5/§13).
func (app *application) handleClientMsg(c *client, room string, m Message) {
	switch m.Type {

	// --- Initialisation du sabot (menu init) -------------------------------
	case "init":
		var p payloadInit
		if err := json.Unmarshal(m.Payload, &p); err != nil {
			return
		}
		cards, err := BuildDeck(p.Config)
		if err != nil {
			log.Printf("init: config invalide: %v", err)
			return
		}
		if p.Shuffle {
			ShuffleDeck(cards)
		}
		app.engine.mu.Lock()
		app.engine.LoadDeck(cards)
		app.engine.mu.Unlock()
		log.Printf("init: sabot chargé (%d cartes) par %s", len(cards), c.name)
		app.broadcastState(room)

	// --- Opérations sur une carte de table ---------------------------------
	case "flip":
		var p payloadCardOp
		_ = json.Unmarshal(m.Payload, &p)
		app.engine.mu.Lock()
		ok := app.engine.Flip(p.CardID)
		app.engine.mu.Unlock()
		if ok {
			app.broadcastState(room)
		}

	case "front":
		var p payloadCardOp
		_ = json.Unmarshal(m.Payload, &p)
		app.engine.mu.Lock()
		ok := app.engine.BringToFront(p.CardID)
		app.engine.mu.Unlock()
		if ok {
			app.broadcastState(room)
		}

	case "rotate":
		var p payloadCardOp
		_ = json.Unmarshal(m.Payload, &p)
		app.engine.mu.Lock()
		ok := app.engine.Rotate(p.CardID, p.Rotate)
		app.engine.mu.Unlock()
		_ = ok
		app.broadcastState(room)

	case "move":
		// Drag terminé : repositionnement d'une carte de table.
		var p payloadCardOp
		_ = json.Unmarshal(m.Payload, &p)
		app.engine.mu.Lock()
		ok := app.engine.Move(p.CardID, p.X, p.Y)
		app.engine.mu.Unlock()
		if ok {
			app.broadcastState(room)
		}

	case "drag":
		// Drag en cours (flux live, pour la fluidité inter-clients). On ne
		// verrouille pas l'état : on relaie juste la position aux autres.
		app.broadcastMsg(room, m, c)

	case "transfer":
		var p payloadTransfer
		if err := json.Unmarshal(m.Payload, &p); err != nil {
			return
		}
		t := Transfer{
			CardID:  p.CardID,
			Target:  DropTarget(p.Target),
			X:       p.X,
			Y:       p.Y,
			OwnerID: p.OwnerID,
		}
		app.engine.mu.Lock()
		res := app.engine.TransferCard(t)
		app.engine.mu.Unlock()
		if res.PublicChanged {
			app.broadcastState(room)
		}
		if res.HandOwner != "" {
			app.sendHand(room, res.HandOwner)
		}

	case "sabotDraw":
		// Drag du sabot (§6) : tire le sommet vers une cible.
		var p payloadTransfer
		if err := json.Unmarshal(m.Payload, &p); err != nil {
			return
		}
		t := Transfer{
			CardID:  "", // carte tirée = sommet du sabot
			Target:  DropTarget(p.Target),
			X:       p.X,
			Y:       p.Y,
			OwnerID: p.OwnerID,
		}
		app.engine.mu.Lock()
		_, res := app.engine.DrawSabot(t)
		app.engine.mu.Unlock()
		if res.PublicChanged {
			app.broadcastState(room)
		}
		if res.HandOwner != "" {
			app.sendHand(room, res.HandOwner)
		}

	case "chat":
		var p payloadChat
		_ = json.Unmarshal(m.Payload, &p)
		if strings.TrimSpace(p.Text) == "" {
			return
		}
		// Relay signé : on réinjecte le nom du joueur authentifié (anti-usurpation).
		out := Message{
			Type:    "chat",
			Sender:  c.userID,
			Payload: mustJSON(payloadChatOut{Author: c.name, Text: p.Text, At: time.Now().UnixMilli()}),
		}
		app.broadcastMsg(room, out, nil)
	}
}

// payloadChatOut est le payload renvoyé au client sur un message de chat.
type payloadChatOut struct {
	Author string `json:"author"`
	Text   string `json:"text"`
	At     int64  `json:"at"`
}
