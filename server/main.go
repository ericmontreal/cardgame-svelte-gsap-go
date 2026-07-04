package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type Message struct {
	Type    string          `json:"type"`           // "join" | "chat" | "action" | "state" | "ping"
	Room    string          `json:"room,omitempty"` // nom de la room
	Sender  string          `json:"sender,omitempty"`
	Payload json.RawMessage `json:"payload"`
}

// client représente une connexion WS
type client struct {
	conn *websocket.Conn
	send chan []byte
	room string
	id   string
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

// hub de rooms
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
				// canal plein : drop
			}
		}
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     checkOrigin,
}

// allowedOrigins renvoie la liste blanche des origines autorisées pour le
// WebSocket (protection CSWSH). Par défaut, en dev, on autorise les origines
// locales du front Vite. En production, définir ALLOWED_ORIGINS (séparées par
// des virgules), par ex. "https://cardgame.example.com".
func allowedOrigins() map[string]bool {
	out := map[string]bool{}
	// Defaults (dev Vite + variantes localhost)
	for _, o := range []string{
		"http://localhost:5173", "http://127.0.0.1:5173",
		"http://localhost:4173", "http://127.0.0.1:4173",
	} {
		out[o] = true
	}
	if extra := os.Getenv("ALLOWED_ORIGINS"); extra != "" {
		for _, o := range strings.Split(extra, ",") {
			if o = strings.TrimSpace(o); o != "" {
				out[o] = true
			}
		}
	}
	return out
}

func checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		// Requête non-navigateur (curl, etc.) : on accepte en dev uniquement.
		// En prod, retirez cette branche pour exiger une origine explicite.
		return true
	}
	return allowedOrigins()[origin]
}

func main() {
	h := newHub()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		room := r.URL.Query().Get("room")
		if room == "" {
			room = "lobby"
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("upgrade:", err)
			return
		}
		cli := &client{
			conn: conn,
			send: make(chan []byte, 256),
			id:   nextClientID(),
		}
		h.join(cli, room)
		log.Printf("client %s joined room %s\n", cli.id, room)

		// writer
		go func() {
			defer func() {
				conn.Close()
			}()
			for msg := range cli.send {
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					return
				}
			}
		}()

		// reader
		// Ping/pong natif WebSocket : ferme automatiquement une connexion
		// muette (délai de lecture + heartbeat), ce qui nettoie le hub.
		const (
			readDeadline    = 70 * time.Second
			heartbeatPeriod = 30 * time.Second
		)
		conn.SetReadDeadline(time.Now().Add(readDeadline))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(readDeadline))
			return nil
		})
		go func() {
			ticker := time.NewTicker(heartbeatPeriod)
			defer ticker.Stop()
			for range ticker.C {
				if err := conn.WriteControl(
					websocket.PingMessage,
					nil,
					time.Now().Add(10*time.Second),
				); err != nil {
					return
				}
			}
		}()

		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				break
			}
			// ping -> pong rapide (non bloquant : on ne s'accroche pas si le
			// canal d'envoi est saturé, contrairement à un envoi direct).
			var m Message
			if err := json.Unmarshal(data, &m); err == nil && m.Type == "ping" {
				pong := Message{Type: "pong", Room: room}
				out, _ := json.Marshal(pong)
				select {
				case cli.send <- out:
				default:
				}
				continue
			}
			// Relay à la room
			h.broadcast(room, data, cli)
		}
		// fermeture
		h.leave(cli)
		close(cli.send)
		_ = conn.Close()
		log.Printf("client %s left room %s\n", cli.id, room)
	})

	log.Println("WS server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
