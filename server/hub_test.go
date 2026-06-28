package main

import (
	"bytes"
	"sync"
	"testing"
)

// newTestClient crée un client sans connexion réelle : seul le canal d'envoi
// (bufférisé) nous intéresse pour vérifier ce que le hub broadcast.
func newTestClient(room string) *client {
	return &client{
		send: make(chan []byte, 16),
		room: room,
	}
}

func TestJoinLeaveBroadcast(t *testing.T) {
	h := newHub()

	a := newTestClient("lobby")
	b := newTestClient("lobby")

	h.join(a, "lobby")
	h.join(b, "lobby")

	// Deux clients dans la room "lobby"
	if got := len(h.rooms["lobby"]); got != 2 {
		t.Fatalf("lobby devrait contenir 2 clients, en a %d", got)
	}

	// a broadcast à la room sauf lui-même : b doit recevoir.
	payload := []byte(`{"type":"chat","payload":{"text":"hi"}}`)
	h.broadcast("lobby", payload, a)

	select {
	case got := <-b.send:
		if !bytes.Equal(got, payload) {
			t.Fatalf("b a reçu %q, attendait %q", got, payload)
		}
	default:
		t.Fatal("b n'a rien reçu du broadcast")
	}

	// a ne reçoit pas son propre message (except == a)
	select {
	case got := <-a.send:
		t.Fatalf("a ne devrait pas recevoir son propre broadcast: %q", got)
	default:
		// OK
	}

	// leave d'un client : la room doit décrémenter.
	h.leave(a)
	if got := len(h.rooms["lobby"]); got != 1 {
		t.Fatalf("lobby devrait contenir 1 client après leave, en a %d", got)
	}
}

func TestLeaveEmptiesRoom(t *testing.T) {
	h := newHub()
	c := newTestClient("solo")
	h.join(c, "solo")
	h.leave(c)

	// La room vide doit être supprimée du hub.
	h.mu.RLock()
	_, exists := h.rooms["solo"]
	h.mu.RUnlock()
	if exists {
		t.Fatal("la room 'solo' vide devrait être supprimée du hub")
	}
}

func TestBroadcastIsolatedPerRoom(t *testing.T) {
	h := newHub()
	lobbyA := newTestClient("lobby")
	other := newTestClient("other")
	h.join(lobbyA, "lobby")
	h.join(other, "other")

	h.broadcast("lobby", []byte(`{"type":"x"}`), nil)

	// Le client de la room 'other' ne doit rien recevoir.
	select {
	case got := <-other.send:
		t.Fatalf("le client 'other' ne devrait pas recevoir un broadcast 'lobby': %q", got)
	default:
		// OK
	}

	// Le client 'lobby' reçoit.
	select {
	case <-lobbyA.send:
		// OK
	default:
		t.Fatal("le client 'lobby' devrait recevoir le broadcast")
	}
}

func TestConcurrentJoinLeave(t *testing.T) {
	// Sanity check : le hub doit rester cohérent sous accès concurrents.
	h := newHub()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			c := newTestClient("lobby")
			h.join(c, "lobby")
			h.leave(c)
		}(i)
	}
	wg.Wait()

	h.mu.RLock()
	count := len(h.rooms["lobby"])
	h.mu.RUnlock()
	if count != 0 {
		t.Fatalf("lobby devrait être vide après join/leave concurrents, contient %d", count)
	}
}

func TestNextClientIDUnique(t *testing.T) {
	// Les IDs doivent être uniques même lorsqu'ils sont générés en rafale.
	const n = 1000
	seen := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		id := nextClientID()
		if seen[id] {
			t.Fatalf("ID en doublon: %s", id)
		}
		seen[id] = true
	}
}
