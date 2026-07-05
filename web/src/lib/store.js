// Store réactif Svelte : miroir local de l'état serveur autoritaire.
//
// Une seule source de vérité (le serveur) ; ce store ne fait qu'appliquer les
// snapshots/deltas reçus via le WebSocket. Aucune logique de jeu ici.

import { writable } from 'svelte/store'

// ---- Session utilisateur (persistance locale du token) -------------------

const SESSION_KEY = 'cardgame.session'

export function loadSession() {
  try {
    const raw = localStorage.getItem(SESSION_KEY)
    if (!raw) return null
    const s = JSON.parse(raw)
    if (s && s.token && s.id && s.name) return s
  } catch {}
  return null
}

export function saveSession(s) {
  localStorage.setItem(SESSION_KEY, JSON.stringify(s))
}

export function clearSession() {
  localStorage.removeItem(SESSION_KEY)
}

// ---- Configuration du sabot (persistance locale du menu init) ------------
//
// Conservée entre les parties (§ menu init affiché à chaque nouvelle partie)
// tant que l'utilisateur ne clique pas sur "Réinitialiser".

const DECK_CONFIG_KEY = 'cardgame.deckConfig'

export function loadDeckConfig() {
  try {
    const raw = localStorage.getItem(DECK_CONFIG_KEY)
    if (!raw) return null
    return JSON.parse(raw)
  } catch {}
  return null
}

export function saveDeckConfig(cfg) {
  localStorage.setItem(DECK_CONFIG_KEY, JSON.stringify(cfg))
}

export function clearDeckConfig() {
  localStorage.removeItem(DECK_CONFIG_KEY)
}

// ---- Auth : POST /api/login ----------------------------------------------

export async function login(username, password) {
  const res = await fetch('/api/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  })
  if (!res.ok) {
    const txt = await res.text().catch(() => '')
    throw new Error(`Identifiants invalides (${res.status}) ${txt}`)
  }
  return res.json() // { token, id, name }
}

// ---- État partagé de la table -------------------------------------------

// État initial : table vide, aucun joueur.
function emptyState() {
  return {
    initialized: false,
    sabotCount: 0,
    table: [],     // cartes publiques sur le tapis
    players: [],   // joueurs connectés (avec position d'avatar)
  }
}

export const tableState = writable(emptyState())

// Main privée du joueur courant (cartes dont il est propriétaire).
export const myHand = writable([])

// File de messages du chat (non persistés : disparition au reload, comme
// l'état de partie — cohérent avec §4/§9).
export const chatLog = writable([])

// Statut de la connexion WebSocket.
export const wsStatus = writable('idle')

// réinitialise complètement l'état local (logout / déconnexion).
export function resetLocal() {
  tableState.set(emptyState())
  myHand.set([])
  chatLog.set([])
  wsStatus.set('idle')
}

// ---- Helpers de mise à jour réactive ------------------------------------

// Applique un message serveur de type "state" (snapshot public complet).
export function applyState(payload) {
  tableState.set({
    initialized: !!payload.initialized,
    sabotCount: payload.sabotCount ?? 0,
    table: Array.isArray(payload.table) ? payload.table : [],
    players: Array.isArray(payload.players) ? payload.players : [],
  })
}

// Applique un message "hand" (main privée du joueur courant).
export function applyHand(payload) {
  myHand.set(Array.isArray(payload.cards) ? payload.cards : [])
}

// Ajoute un message de chat au journal.
export function applyChat(msg) {
  const entry = {
    author: msg?.payload?.author ?? msg?.sender ?? '?',
    text: String(msg?.payload?.text ?? ''),
    at: msg?.payload?.at ?? Date.now(),
  }
  chatLog.update((log) => [...log, entry].slice(-200))
}

// Carte en cours de déplacement par un autre joueur (drag live). On garde
// l'ID en clé pour que la vue s'update en temps réel sans toucher au store
// autoritaire (qui ne change qu'au drop).
export const liveDrag = writable(null) // { cardId, x, y, who } | null
