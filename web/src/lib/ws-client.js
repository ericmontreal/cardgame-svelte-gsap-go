// Client WebSocket minimal pour le serveur Go du projet.
//
// Convention de messages (compatible avec le relay/broadcast actuel du serveur) :
//   { type: 'join',  room, sender }
//   { type: 'chat',  room, sender, payload: { text } }
//   { type: 'deal',  room, sender, payload: { hands, perPlayer } }  // synchro d'un deal
//   { type: 'state', room, sender, payload: { ... } }               // état libre (réservé)
//   { type: 'ping' } / { type: 'pong' }                              // keep-alive géré ici
//
// Le serveur actuel se contente de relayer à la room (sauf l'émetteur),
// donc la "synchro" est coopérative : un client émet un 'deal', les autres
// l'appliquent. Ce n'est pas un état autoritaire — voir README.

const DEFAULTS = {
  url: 'ws://localhost:8080/ws',
  room: 'lobby',
  pingInterval: 25000,   // ms
  reconnectDelay: 1000,  // ms, délai initial puis backoff
  maxReconnectDelay: 15000,
  maxReconnectAttempts: 0, // 0 = infini
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

  function connect() {
    if (manuallyClosed) return
    const url = `${cfg.url}?room=${encodeURIComponent(cfg.room)}`
    emitStatus('connecting')
    try {
      ws = new WebSocket(url)
    } catch (e) {
      emitStatus('error', { error: e })
      scheduleReconnect()
      return
    }

    ws.addEventListener('open', () => {
      // Annonce de présence dans la room
      send({ type: 'join', room: cfg.room })
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
      // pong interne : on ne propage pas
      if (msg.type === 'pong') return
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

  return {
    connect,
    close,
    send,
    // Envoi typé pratique
    sendDeal(hands, perPlayer) {
      return send({ type: 'deal', room: cfg.room, payload: { hands, perPlayer } })
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
  }
}
