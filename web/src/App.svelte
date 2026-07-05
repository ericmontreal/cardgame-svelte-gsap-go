<script>
  // Application principale : layout 3 zones (§7), gate d'auth (§10), menu init
  // (§4), câblage WebSocket avec le serveur autoritaire (§5/§12).
  import { onMount, onDestroy } from 'svelte'
  import { gsap } from 'gsap'

  import {
    tableState, myHand, wsStatus,
    applyState, applyHand, applyChat, liveDrag,
    loadSession, saveSession, clearSession, resetLocal, login,
  } from './lib/store.js'
  import { createWsClient } from './lib/ws-client.js'
  import { ensureInlineSprite } from './lib/svg-sprite.js'

  import Login from './lib/Login.svelte'
  import InitMenu from './lib/InitMenu.svelte'
  import Table from './lib/Table.svelte'
  import Hand from './lib/Hand.svelte'
  import Chat from './lib/Chat.svelte'

  // ---- Étapes d'application ----
  // auth -> connecting -> (init | table). La session est restaurée depuis
  // localStorage. "connecting" attend le premier snapshot d'état du serveur
  // pour savoir si une partie est déjà en cours (une seule pièce, le lobby) :
  // si oui, on rejoint directement la table plutôt que de rouvrir le menu
  // init (qui, soumis, remplacerait le sabot en cours pour tout le monde).
  let session = loadSession()
  let step = session ? 'connecting' : 'auth'
  let joinDecided = false
  let errorMsg = ''

  // Aide de connexion : comptes démo si USERS_SEED non défini côté serveur.
  const SEED_HINT = 'Comptes démo : alice/secret, bob/secret'

  // ---- Client WebSocket ----
  let ws = null
  let unsubMsg = null
  let unsubStatus = null

  function connectWs() {
    if (ws) return
    ws = createWsClient({ token: session.token })
    unsubMsg = ws.on(handleServerMsg)
    unsubStatus = ws.onStatus((s) => wsStatus.set(s))
    ws.connect()
  }

  function disconnectWs() {
    if (unsubMsg) { unsubMsg(); unsubMsg = null }
    if (unsubStatus) { unsubStatus(); unsubStatus = null }
    if (ws) { ws.close(); ws = null }
  }

  // ---- Réception des messages serveur ----
  function handleServerMsg(msg) {
    switch (msg.type) {
      case 'state':
        applyState(msg.payload)
        // Premier snapshot reçu après connexion : décide si on rejoint la
        // partie en cours (sabot déjà initialisé) ou si on affiche le menu
        // init (aucune partie en cours). Ne se déclenche qu'une fois par
        // connexion : un "state" reçu plus tard (pendant qu'on est déjà sur
        // le menu "Nouvelle partie", par ex.) ne doit pas nous en éjecter.
        if (!joinDecided) {
          joinDecided = true
          step = msg.payload?.initialized ? 'table' : 'init'
        }
        break
      case 'hand':
        applyHand(msg.payload)
        break
      case 'chat':
        applyChat(msg)
        break
      case 'drag':
        // Drag live d'un autre joueur : on met à jour la position éphémère.
        if (msg.payload) {
          liveDrag.set({ cardId: msg.payload.cardId, x: msg.payload.x, y: msg.payload.y })
        }
        break
    }
  }

  // ---- Auth ----
  function onLoginSuccess(e) {
    session = e.detail
    saveSession(session)
    step = 'connecting'
    connectWs()
  }

  // ---- Menu init -> ouverture de la table ----
  // La WS est déjà connectée et ouverte à ce stade (on n'atteint 'init' qu'après
  // avoir reçu le premier "state" du serveur sur cette connexion).
  function onInitStart(e) {
    const { config, shuffle } = e.detail
    ws?.sendInit(config, shuffle)
    step = 'table'
  }

  // ---- Actions de table (remontées des composants) -> serveur ----
  function sendMove(e)    { ws?.sendMove(e.detail.cardId, e.detail.x, e.detail.y) }
  function sendDrag(e)    { ws?.sendDrag(e.detail.cardId, e.detail.x, e.detail.y) }
  function sendFlip(e)    { ws?.sendFlip(e.detail.cardId) }
  function sendFront(e)   { ws?.sendFront(e.detail.cardId) }
  function sendTransfer(e) {
    const d = e.detail
    ws?.sendTransfer(d.cardId, d.target, d.x ?? 0, d.y ?? 0, d.ownerId ?? '')
  }
  function sendSabotDraw(e) {
    const d = e.detail
    ws?.sendSabotDraw(d.target, d.x ?? 0, d.y ?? 0, d.ownerId ?? '')
  }
  function sendChat(text) { ws?.sendChat(text) }

  // ---- Nouvelle partie : réaffiche le menu de config (§ menu init à chaque
  // nouvelle partie). La session/connexion WS reste active ; seul le sabot
  // sera remplacé au prochain "Ouvrir la table".
  function newGame() {
    step = 'init'
  }

  // ---- Déconnexion ----
  function logout() {
    disconnectWs()
    clearSession()
    resetLocal()
    session = null
    joinDecided = false
    step = 'auth'
  }

  // ---- Cycle de vie : chargement du sprite de cartes ----
  onMount(async () => {
    try { await ensureInlineSprite('/cards.svg') } catch (e) { console.warn(e) }
    // Session déjà connue (retour sur l'app) : se connecte tout de suite pour
    // pouvoir décider init/table dès le premier état reçu.
    if (session) connectWs()
  })
  onDestroy(() => disconnectWs())

  // Statut lisible pour la barre supérieure.
  $: statusLabel = {
    idle: 'déconnecté', connecting: 'connexion…', open: 'connecté',
    reconnecting: 'reconnexion…', closed: 'déconnecté', error: 'erreur',
  }[$wsStatus] || $wsStatus

  // État réactif partagé (tableState + myHand) pour les sous-composants.
  $: st = $tableState
  $: hand = $myHand
</script>

<main class="app">
  {#if step === 'auth'}
    <Login seedHint={SEED_HINT} on:success={onLoginSuccess} />

  {:else if step === 'connecting'}
    <div class="connecting">Connexion à la table…</div>

  {:else if step === 'init'}
    <InitMenu seedHint={SEED_HINT} on:start={onInitStart} />

  {:else}
    <div class="layout">
      <header class="topbar">
        <span class="brand">🃏 Table de cartes</span>
        <span class="me">Connecté en tant que <b>{session.name}</b></span>
        <span class="status" data-s={$wsStatus}>● {statusLabel}</span>
        <button class="newgame" on:click={newGame}>Nouvelle partie</button>
        <button class="logout" on:click={logout}>Déconnexion</button>
      </header>

      <div class="body">
        <Table
          table={st.table}
          players={st.players}
          sabotCount={st.sabotCount}
          initialized={st.initialized}
          myUserId={session.id}
          on:move={sendMove}
          on:drag={sendDrag}
          on:flip={sendFlip}
          on:front={sendFront}
          on:transfer={sendTransfer}
          on:sabotDraw={sendSabotDraw}
        />

        <Chat onSend={sendChat} />
      </div>

      <Hand {hand} myUserId={session.id}
        on:flip={sendFlip}
        on:front={sendFront}
        on:transfer={sendTransfer}
      />
    </div>
  {/if}
</main>

<style>
  :global(html, body, #app) { height: 100%; margin: 0; }
  :global(body) { background: #062016; color: #eef; font-family: system-ui, sans-serif; }

  .app { height: 100vh; display: flex; flex-direction: column; }

  .connecting {
    min-height: 100vh; display: grid; place-items: center;
    background: radial-gradient(circle at 50% 30%, #1c6e4b 0%, #0d3a26 70%, #062016 100%);
    color: #eef; font-family: system-ui, sans-serif; opacity: .85;
  }

  .layout { flex: 1; display: grid; grid-template-rows: auto 1fr auto; min-height: 0; }

  .topbar {
    display: flex; align-items: center; gap: 1rem;
    padding: 8px 14px;
    background: rgba(0,0,0,0.45);
    border-bottom: 2px solid rgba(255,255,255,0.1);
    font-size: .9rem;
  }
  .brand { font-weight: 700; }
  .me { opacity: .85; }
  .me b { color: #ffd27a; }
  .status { margin-left: auto; font-size: .8rem; opacity: .85; }
  .status[data-s="open"] { color: #6fe39a; }
  .status[data-s="connecting"], .status[data-s="reconnecting"] { color: #ffd27a; }
  .status[data-s="closed"], .status[data-s="error"], .status[data-s="idle"] { color: #ff9a9a; }
  .newgame, .logout {
    border: 1px solid rgba(255,255,255,0.2); background: transparent; color: #eef;
    padding: .35rem .7rem; border-radius: 7px; cursor: pointer; font-size: .85rem;
  }
  .newgame:hover, .logout:hover { background: rgba(255,255,255,0.1); }

  .body { display: grid; grid-template-columns: 1fr auto; min-height: 0; }
</style>
