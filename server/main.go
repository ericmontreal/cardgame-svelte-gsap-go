package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
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

// hub de rooms
type hub struct {
	mu     sync.RWMutex
	rooms  map[string]map[*client]struct{}
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
	if c.room == "" { return }
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
	CheckOrigin: func(r *http.Request) bool {
		// En prod, restreindre l'origine ici
		return true
	},
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
			id:   time.Now().Format("150405.000"),
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
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				break
			}
			// ping -> pong rapide
			var m Message
			if err := json.Unmarshal(data, &m); err == nil && m.Type == "ping" {
				pong := Message{Type: "pong", Room: room}
				out, _ := json.Marshal(pong)
				cli.send <- out
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
