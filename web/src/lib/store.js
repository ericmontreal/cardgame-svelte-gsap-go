// Store réactif Svelte : miroir local de l'état serveur autoritaire.
//
// Une seule source de vérité (le serveur) ; ce store ne fait qu'appliquer les
// snapshots/deltas reçus via le WebSocket. Aucune logique de jeu ici.

import { writable, get } from 'svelte/store'

// ---- Session utilisateur (persistance par onglet du token) ---------------
//
// sessionStorage (et non localStorage) : chaque onglet/fenêtre gère sa propre
// session. Avec localStorage, ouvrir une deuxième fenêtre pour tester un
// second joueur reprenait automatiquement le compte de la première (stockage
// partagé entre tous les onglets d'un même navigateur), obligeant à se
// déconnecter manuellement pour changer de compte — et pire, deux connexions
// WS simultanées sous le même compte, dont la fermeture de l'une supprimait
// l'avatar de l'autre côté serveur (§ voir hub.go/countUser).

const SESSION_KEY = 'cardgame.session'

export function loadSession() {
  try {
    const raw = sessionStorage.getItem(SESSION_KEY)
    if (!raw) return null
    const s = JSON.parse(raw)
    if (s && s.token && s.id && s.name) return s
  } catch {}
  return null
}

export function saveSession(s) {
  sessionStorage.setItem(SESSION_KEY, JSON.stringify(s))
}

export function clearSession() {
  sessionStorage.removeItem(SESSION_KEY)
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

// ---- Dernier identifiant saisi (confort de reconnexion) ------------------
//
// localStorage et non sessionStorage, contrairement au token ci-dessus : le
// but est justement de survivre à la fermeture du navigateur. Seul le nom est
// conservé, jamais le mot de passe, et il n'est enregistré qu'après une
// connexion réussie — une faute de frappe ne se réinstalle donc pas.
//
// Conséquence assumée : ouvrir une seconde fenêtre pour jouer un autre compte
// pré-remplit le nom du premier. Le champ reste modifiable, et rien n'est
// connecté automatiquement.

const LAST_USER_KEY = 'cardgame.lastUsername'

export function loadLastUsername() {
  try {
    return localStorage.getItem(LAST_USER_KEY) || ''
  } catch {}
  return ''
}

export function saveLastUsername(name) {
  try {
    localStorage.setItem(LAST_USER_KEY, name)
  } catch {}
}

// ---- Chat replié ou non (préférence locale) ------------------------------
//
// Conservée d'une session à l'autre : un joueur qui joue sur petit écran replie
// le chat une fois, pas à chaque partie. Purement locale, jamais envoyée au
// serveur — chacun voit la table comme il l'entend.

const CHAT_HIDDEN_KEY = 'cardgame.chatHidden'

export function loadChatHidden() {
  try {
    return localStorage.getItem(CHAT_HIDDEN_KEY) === '1'
  } catch {}
  return false
}

export function saveChatHidden(hidden) {
  try {
    localStorage.setItem(CHAT_HIDDEN_KEY, hidden ? '1' : '0')
  } catch {}
}

// ---- Niveau de performance (préférence locale) ---------------------------
//
// « high » (défaut, comportement d'origine) : les gestes en cours sont diffusés
// en continu et ceux des autres affichés ; animations complètes.
// « low » : on n'envoie plus la position de ses cartes pendant le geste (le
// résultat part au lâcher), le serveur cesse de nous relayer les gestes des
// autres, et les animations décoratives sont coupées. Allège la tablette et la
// connexion montante du serveur. Retenu sur l'appareil, comme le repli du chat.

const PERF_KEY = 'cardgame.perf'

function loadPerf() {
  try {
    return localStorage.getItem(PERF_KEY) === 'low' ? 'low' : 'high'
  } catch {}
  return 'high'
}

export const perfLevel = writable(loadPerf())
perfLevel.subscribe((v) => {
  try { localStorage.setItem(PERF_KEY, v) } catch {}
})

// Raccourci pour le code non réactif (gestionnaires d'événements, animations).
export function lowPerf() {
  return get(perfLevel) === 'low'
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

// isSessionValid vérifie auprès du serveur qu'un jeton mémorisé vaut encore
// quelque chose. À appeler AVANT d'ouvrir la WebSocket : un jeton périmé la
// fait échouer sans que le navigateur en révèle la cause (aucun code HTTP n'est
// exposé sur un échec de poignée de main), et le client boucle alors sur sa
// reconnexion, figé sur l'écran d'attente.
//
// En cas de serveur injoignable on renvoie true : le jeton n'est pas en cause,
// et la reconnexion automatique de la WebSocket reste la bonne réponse.
export async function isSessionValid(token) {
  if (!token) return false
  try {
    const res = await fetch('/api/session', {
      headers: { Authorization: `Bearer ${token}` },
    })
    if (res.status === 401) return false
    return true
  } catch {
    return true
  }
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
  liveDrag.set(null)
  sabotShuffle.set(null)
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
  // Un snapshot autoritaire fait toujours foi : on efface toute position de
  // drag "live" résiduelle, sinon une carte reste figée à sa dernière
  // position de survol une fois le glissement terminé (jamais réinitialisée
  // sinon), désynchronisée du véritable état serveur pour les autres clients.
  liveDrag.set(null)
}

// Applique un message "hand" (main privée du joueur courant).
//
// Ordre d'affichage : de gauche à droite par ordre d'arrivée. Les cartes déjà
// en main gardent leur place, les nouvelles s'ajoutent à droite, dans l'ordre
// où le serveur les envoie. Le serveur trie lui aussi par arrivée (Z) ; garder
// l'ordre ici évite en plus qu'une carte retournée ou reprise ne fasse
// bouger ses voisines.
export function applyHand(payload) {
  const cards = Array.isArray(payload?.cards) ? payload.cards : []
  myHand.update((prev) => mergeHandOrder(prev, cards))
}

// Réordonne la main localement (glisser une carte dans sa propre main), sans
// attendre le serveur : l'ordre est d'abord une affaire d'affichage.
export function reorderHand(cardIds) {
  myHand.update((cards) => {
    const byId = new Map(cards.map((c) => [c.id, c]))
    const listed = cardIds.filter((id) => byId.has(id)).map((id) => byId.get(id))
    const rest = cards.filter((c) => !cardIds.includes(c.id))
    return [...listed, ...rest]
  })
}

export function mergeHandOrder(prev, cards) {
  const place = new Map(prev.map((c, i) => [c.id, i]))
  const kept = cards.filter((c) => place.has(c.id)).sort((a, b) => place.get(a.id) - place.get(b.id))
  const added = cards.filter((c) => !place.has(c.id))
  return [...kept, ...added]
}

// Ajoute un message de chat au journal.
export function applyChat(msg) {
  const entry = {
    id: ++chatSeq,
    author: msg?.payload?.author ?? msg?.sender ?? '?',
    text: String(msg?.payload?.text ?? ''),
    at: msg?.payload?.at ?? Date.now(),
  }
  chatLog.update((log) => [...log, entry].slice(-200))
}

// Dernier mélange du sabot annoncé par le serveur : { by, count, at } | null.
// Le sabot s'y abonne pour jouer son animation chez TOUS les joueurs, pas
// seulement chez celui qui a cliqué — l'état public, lui, ne change pas (seul
// le nombre de cartes du sabot est connu), il ne peut donc pas servir de signal.
export const sabotShuffle = writable(null)

// Applique un message "sabotShuffled" : déclenche l'animation et laisse une
// trace dans le chat, pour qu'un joueur qui regardait ailleurs sache que le
// paquet a été battu, et par qui.
export function applySabotShuffled(msg) {
  const by = String(msg?.payload?.by ?? '?')
  const count = Number(msg?.payload?.count) || 0
  const at = Date.now()
  sabotShuffle.set({ by, count, at })
  pushTableNote(`${by} mélange le sabot (${cardsLabel(count)})`, at) // même format compact que les distributions
}

// Applique un message "cardsReceived" : des cartes sont entrées dans une main.
export function applyCardsReceived(msg) {
  const text = receivedText(msg?.payload)
  if (text) pushTableNote(text)
}

// receivedText rédige l'annonce d'une réception de cartes, sous forme
// compacte « auteur verbe … » — l'auteur du geste en tête. Elle ne nomme
// jamais les cartes : le serveur ne les transmet pas.
//   alice distribue une carte à bob   (depuis le sabot)
//   alice donne 2 cartes à bob        (depuis le tapis ou sa main)
//   alice tire une carte              (du sabot, pour elle-même)
//   alice ramasse une carte           (du tapis, pour elle-même)
export function receivedText(p) {
  const by = String(p?.by ?? '?')
  const to = String(p?.to ?? '?')
  const n = Number(p?.count) || 0
  if (n <= 0) return ''
  const cards = n === 1 ? 'une carte' : `${n} cartes`
  if (p?.self) {
    switch (p.from) {
      case 'sabot': return `${to} tire ${cards}`
      case 'table': return `${to} ramasse ${cards}`
      default: return `${to} prend ${cards}`
    }
  }
  const verb = p?.from === 'sabot' ? 'distribue' : 'donne'
  return `${by} ${verb} ${cards} à ${to}`
}

function cardsLabel(n) {
  return `${n} carte${n > 1 ? 's' : ''}`
}

// Annonce de la table dans le chat : rendue à part (italique, sans auteur),
// jamais saisie par un joueur.
function pushTableNote(text, at = Date.now()) {
  chatLog.update((log) => [...log, { id: ++chatSeq, author: '', system: true, text, at }].slice(-200))
}

// Identifiant de ligne du chat, clé du {#each} de Chat.svelte. Le triplet
// heure|auteur|texte ne suffit plus : deux tirages dans la même milliseconde
// produisent deux annonces identiques, et Svelte refuse les clés en double.
let chatSeq = 0

// Cartes en cours de déplacement par un autre joueur (drag live, simple ou
// groupé). Dictionnaire cardId -> {x,y} pour que la vue s'update en temps réel
// sans toucher au store autoritaire (qui ne change qu'au drop). Chaque message
// live remplace intégralement le précédent (un seul drag à la fois par table).
export const liveDrag = writable(null) // { [cardId]: {x,y} } | null
