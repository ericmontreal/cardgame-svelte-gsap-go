package main

import (
	"encoding/json"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// Message est la convention d'échange client/serveur (JSON sur WebSocket).
type Message struct {
	Type    string          `json:"type"`           // "init" | "flip" | "front" | "rotate" | "drop" | "drag" | "sabotMove" | "chat" | "ping" | ...
	Room    string          `json:"room,omitempty"` // room courante (renvoyée par le serveur)
	Sender  string          `json:"sender,omitempty"`
	Payload json.RawMessage `json:"payload"`
}

// client représente une connexion WS authentifiée.
type client struct {
	conn   *websocket.Conn
	send   chan []byte
	room   string
	id     string // identifiant technique (logs)
	userID string // identifiant de l'utilisateur authentifié
	name   string // nom affiché (avatar, chat)
}

// nextClientID génère un identifiant unique par incrémentation atomique. On
// évite ainsi les collisions de time.Now().Format(...) lorsque deux clients
// se connectent dans la même milliseconde. Préfixe de date pour la lisibilité
// des logs.
var clientCounter atomic.Uint64

func nextClientID() string {
	n := clientCounter.Add(1)
	return time.Now().Format("20060102") + "-" + strconv.FormatUint(n, 36)
}

// hub gère l'ensemble des connexions par room et la diffusion des messages.
// Il reste générique (rooms multiples) ; l'application n'en utilise qu'une
// ("lobby"), la table étant unique et globale.
type hub struct {
	mu    sync.RWMutex
	rooms map[string]map[*client]struct{}
}

func newHub() *hub {
	return &hub{rooms: make(map[string]map[*client]struct{})}
}

func (h *hub) join(c *client, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.rooms[room]; !ok {
		h.rooms[room] = make(map[*client]struct{})
	}
	h.rooms[room][c] = struct{}{}
	c.room = room
}

func (h *hub) leave(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c.room == "" {
		return
	}
	if set, ok := h.rooms[c.room]; ok {
		delete(set, c)
		if len(set) == 0 {
			delete(h.rooms, c.room)
		}
	}
}

// broadcast envoie data à tous les clients de la room, sauf `except`.
func (h *hub) broadcast(room string, data []byte, except *client) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if set, ok := h.rooms[room]; ok {
		for cli := range set {
			if cli == except {
				continue
			}
			select {
			case cli.send <- data:
			default:
				// canal plein : drop (le client rattrapera l'état via la
				// diffusion complète envoyée à chaque action mutante)
			}
		}
	}
}

// sendTo envoie data à un client précis (ex. main privée d'un joueur).
func (h *hub) sendTo(c *client, data []byte) {
	select {
	case c.send <- data:
	default:
	}
}

// countUser renvoie le nombre de connexions actives d'un userID dans une room.
// Un même compte peut avoir plusieurs connexions simultanées (plusieurs
// onglets/fenêtres) ; on ne doit retirer son avatar du jeu qu'à la fermeture
// de la toute dernière (voir removePlayer dans main.go).
func (h *hub) countUser(room, userID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	n := 0
	for c := range h.rooms[room] {
		if c.userID == userID {
			n++
		}
	}
	return n
}

// clients renvoie une capture des clients d'une room (pour itérer hors-lock).
func (h *hub) clients(room string) []*client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	set := h.rooms[room]
	out := make([]*client, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	return out
}
