// Client WebSocket pour le serveur Go autoritaire du projet de table de cartes.
//
// Convention de messages (état serveur = source de vérité, §5) :
//   E : émetteur   R : recepteur
//
//   client -> serveur
//     { type:'init',      payload:{ config, shuffle } }           prépare le sabot (menu init)
//     { type:'flip',      payload:{ cardId } }                     retourne une carte
//     { type:'front',     payload:{ cardId } }                     amène au premier plan (clic)
//     { type:'rotate',    payload:{ cardId, rotate } }             rotation
//     { type:'move',      payload:{ cardId, x, y } }               drop sur la table
//     { type:'drag',      payload:{ cardId, x, y } }               position live (fluidité)
//     { type:'transfer',  payload:{ cardId, target, x, y, ownerId } }  change de zone
//     { type:'sabotDraw', payload:{ target, x, y, ownerId } }      tire le sommet du sabot
//     { type:'chat',      payload:{ text } }
//     { type:'ping' }
//
//   serveur -> client
//     { type:'state',   payload:{ sabotCount, table[], players[], initialized } }
//     { type:'hand',    payload:{ cards[] } }    (main privée, propriétaire seul)
//     { type:'drag',    payload:{ cardId, x, y } } (drag live d'un autre client)
//     { type:'chat',    sender, payload:{ author, text, at } }
//     { type:'pong' }

const DEFAULTS = {
  url: 'ws://localhost:8080/ws',
  room: 'lobby',
  token: '',                 // token de session (POST /api/login)
  pingInterval: 25000,       // ms
  reconnectDelay: 1000,      // ms, délai initial puis backoff exponentiel
  maxReconnectDelay: 15000,
  maxReconnectAttempts: 0,   // 0 = infini
}

export function createWsClient(opts = {}) {
  const cfg = { ...DEFAULTS, ...opts }

  let ws = null
  let attempt = 0
  let reconnectTimer = null
  let pingTimer = null
  let manuallyClosed = false
  const listeners = new Set()        // handlers de message entrant
  const statusListeners = new Set()  // handlers de changement de statut

  function emitStatus(status, extra = {}) {
    for (const fn of statusListeners) fn(status, extra)
  }

  function setStatusOpen() {
    attempt = 0
    emitStatus('open')
    startPing()
  }

  function scheduleReconnect() {
    if (manuallyClosed) return
    if (cfg.maxReconnectAttempts && attempt >= cfg.maxReconnectAttempts) {
      emitStatus('closed', { reason: 'max-attempts' })
      return
    }
    const delay = Math.min(cfg.reconnectDelay * Math.pow(2, attempt), cfg.maxReconnectDelay)
    attempt++
    emitStatus('reconnecting', { attempt, delay })
    reconnectTimer = setTimeout(connect, delay)
  }

  function startPing() {
    stopPing()
    pingTimer = setInterval(() => send({ type: 'ping' }), cfg.pingInterval)
  }

  function stopPing() {
    if (pingTimer) { clearInterval(pingTimer); pingTimer = null }
  }

  function buildUrl() {
    const u = new URL(cfg.url)
    u.searchParams.set('room', cfg.room)
    if (cfg.token) u.searchParams.set('token', cfg.token)
    return u.toString()
  }

  function connect() {
    if (manuallyClosed) return
    if (!cfg.token) {
      // Sans token, la connexion serait rejetée par le serveur (401).
      emitStatus('error', { error: 'missing token' })
      return
    }
    emitStatus('connecting')
    try {
      ws = new WebSocket(buildUrl())
    } catch (e) {
      emitStatus('error', { error: e })
      scheduleReconnect()
      return
    }

    ws.addEventListener('open', () => {
      // Pas de 'join' : le serveur enregistre le joueur à l'upgrade (token).
      setStatusOpen()
    })

    ws.addEventListener('message', (ev) => {
      let msg
      try {
        msg = JSON.parse(ev.data)
      } catch {
        return // message non JSON ignoré
      }
      if (!msg || typeof msg !== 'object') return
      if (msg.type === 'pong') return  // keep-alive interne, non propagé
      for (const fn of listeners) fn(msg)
    })

    ws.addEventListener('error', () => {
      emitStatus('error')
    })

    ws.addEventListener('close', () => {
      stopPing()
      if (manuallyClosed) {
        emitStatus('closed', { reason: 'manual' })
        return
      }
      scheduleReconnect()
    })
  }

  function send(obj) {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(obj))
      return true
    }
    return false
  }

  function close() {
    manuallyClosed = true
    stopPing()
    if (reconnectTimer) { clearTimeout(reconnectTimer); reconnectTimer = null }
    if (ws) {
      try { ws.close() } catch {}
      ws = null
    }
    emitStatus('closed', { reason: 'manual' })
  }

  // ---- Envoi typé (wrappers autour de send) ----
  const api = {
    connect,
    close,
    send,

    setToken(token) { cfg.token = token },

    sendInit(config, shuffle = false) {
      return send({ type: 'init', payload: { config, shuffle } })
    },
    sendFlip(cardId) {
      return send({ type: 'flip', payload: { cardId } })
    },
    sendFront(cardId) {
      return send({ type: 'front', payload: { cardId } })
    },
    sendRotate(cardId, rotate) {
      return send({ type: 'rotate', payload: { cardId, rotate } })
    },
    sendMove(cardId, x, y) {
      return send({ type: 'move', payload: { cardId, x, y } })
    },
    sendDrag(cardId, x, y) {
      return send({ type: 'drag', payload: { cardId, x, y } })
    },
    sendTransfer(cardId, target, x, y, ownerId = '') {
      return send({ type: 'transfer', payload: { cardId, target, x, y, ownerId } })
    },
    sendSabotDraw(target, x, y, ownerId = '') {
      return send({ type: 'sabotDraw', payload: { target, x, y, ownerId } })
    },
    sendChat(text) {
      return send({ type: 'chat', payload: { text } })
    },

    // Abonnements. Chaque "on" retourne une fonction de désabonnement.
    on(handler) {
      listeners.add(handler)
      return () => listeners.delete(handler)
    },
    onStatus(handler) {
      statusListeners.add(handler)
      return () => statusListeners.delete(handler)
    },
    get readyState() {
      return ws ? ws.readyState : WebSocket.CLOSED
    },
    get config() {
      return cfg
    },
  }
  return api
}
