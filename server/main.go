package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// application regroupe les dépendances partagées du serveur.
type application struct {
	hub      *hub
	engine   *engine
	users    UserStore
	sessions *sessionManager
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     checkOrigin,
}

// allowedOrigins renvoie la liste blanche des origines autorisées pour le
// WebSocket (protection CSWSH). En dev, on autorise les origines locales du
// front Vite. En production, définir ALLOWED_ORIGINS (séparées par des
// virgules), par ex. "https://cardgame.example.com".
func allowedOrigins() map[string]bool {
	out := map[string]bool{}
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

// authenticateWS valide le paramètre ?token= de la requête WS de升级 et renvoie
// l'utilisateur authentifié. Sans token valide : 401 et pas d'upgrade.
func (app *application) authenticateWS(w http.ResponseWriter, r *http.Request) *User {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		http.Error(w, "unauthorized: missing token", http.StatusUnauthorized)
		return nil
	}
	userID, ok := app.sessions.lookup(token)
	if !ok {
		http.Error(w, "unauthorized: invalid token", http.StatusUnauthorized)
		return nil
	}
	u, ok := app.users.FindByID(userID)
	if !ok {
		http.Error(w, "unauthorized: unknown user", http.StatusUnauthorized)
		return nil
	}
	return u
}

// wsHandler gère une connexion WebSocket authentifiée.
func (app *application) wsHandler(w http.ResponseWriter, r *http.Request) {
	u := app.authenticateWS(w, r)
	if u == nil {
		return
	}
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
		conn:   conn,
		send:   make(chan []byte, 256),
		id:     nextClientID(),
		userID: u.ID,
		name:   u.Name,
	}
	app.hub.join(cli, room)

	// Enregistrement du joueur côté moteur + positionnement de l'avatar.
	app.engine.mu.Lock()
	app.engine.ensurePlayer(u.ID, u.Name, tableW, tableH)
	app.engine.mu.Unlock()

	log.Printf("client %s (%s) joined room %s\n", cli.id, cli.name, room)

	// Annonce de présence : on diffuse l'état public (incluant la liste des
	// joueurs) à toute la room, et la main privée au nouveau venu.
	app.broadcastState(room)
	app.sendHand(room, u.ID)

	// writer : pompe d'envoi vers le client.
	go func() {
		defer conn.Close()
		for msg := range cli.send {
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
	}()

	// reader + heartbeat natif (ping/pong). La fermeture d'une connexion muette
	// nettoie automatiquement le hub et le moteur.
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
				websocket.PingMessage, nil, time.Now().Add(10*time.Second),
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
		var m Message
		if err := json.Unmarshal(data, &m); err != nil {
			continue // message non JSON ignoré
		}
		// ping applicatif -> pong rapide (canal non bloquant).
		if m.Type == "ping" {
			pong := Message{Type: "pong", Room: room}
			out, _ := json.Marshal(pong)
			select {
			case cli.send <- out:
			default:
			}
			continue
		}
		if m.Type == "pong" {
			continue
		}
		// Tout autre message : dispatch autoritaire.
		app.handleClientMsg(cli, room, m)
	}

	// fermeture : retrait du hub + du moteur, puis diffusion de présence.
	app.hub.leave(cli)
	close(cli.send)
	_ = conn.Close()

	app.engine.mu.Lock()
	app.engine.removePlayer(u.ID)
	app.engine.mu.Unlock()
	app.sendPresence(room)

	log.Printf("client %s (%s) left room %s\n", cli.id, cli.name, room)
}

// healthHandler répond 204 pour les sondes de disponibilité (utile en prod).
func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func main() {
	store := newInMemoryUserStore()
	bootstrapUsers(store)

	app := &application{
		hub:      newHub(),
		engine:   newEngine(),
		users:    store,
		sessions: newSessionManager(),
	}

	loginLimiter := newLoginRateLimiter(5, time.Minute)
	http.HandleFunc("/api/login", loginHandler(app.users, app.sessions, loginLimiter))
	http.HandleFunc("/api/health", healthHandler)
	http.HandleFunc("/ws", app.wsHandler)

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("WS server on %s (table logique %vx%v)", addr, tableW, tableH)
	log.Fatal(http.ListenAndServe(addr, nil))
}
