<script>
  // Zone table (§7) : tapis vert partagé. Contient le sabot, les cartes de
  // table (publiques) et les avatars. Orchestre le hit-test des drops et
  // remonte les actions au serveur via events.
  import { createEventDispatcher, onMount } from 'svelte'
  import Card from './Card.svelte'
  import Sabot from './Sabot.svelte'
  import Avatar from './Avatar.svelte'
  import { dropAt, TARGETS } from './drag.js'
  import { liveDrag } from './store.js'

  export let table = []          // cartes publiques (zone table)
  export let players = []        // joueurs connectés
  export let sabotCount = 0
  export let initialized = false
  export let myUserId = ''

  const dispatch = createEventDispatcher()

  let tableEl
  let tableRect = null
  // Position fixe du sabot sur le tapis (zone de design). Le joueur peut le
  // déplacer en le glissant, mais son emplacement par défaut est stable.
  const SABOT_POS = { x: 28, y: 28 }

  function refreshRect() {
    if (tableEl) tableRect = tableEl.getBoundingClientRect()
  }
  onMount(() => {
    refreshRect()
    window.addEventListener('resize', refreshRect)
    window.addEventListener('scroll', refreshRect, true)
    return () => {
      window.removeEventListener('resize', refreshRect)
      window.removeEventListener('scroll', refreshRect, true)
    }
  })
  // Recalcule le rect quand la table change de taille (nouvelles cartes...).
  $: table, players, sabotCount, initialized, refreshRect()

  // ---- Position live d'un drag distant (autre joueur) ----
  let liveCardId = null
  let liveX = 0, liveY = 0
  liveDrag.subscribe((d) => {
    if (!d) { liveCardId = null; return }
    liveCardId = d.cardId
    liveX = d.x
    liveY = d.y
  })

  // ---- Hit-test au drop : route vers la bonne action serveur ----
  function resolveDrop(clientX, clientY, cardId, fromZone) {
    refreshRect()
    const hit = dropAt(clientX, clientY, { tableRect })
    if (!hit) {
      // Hors de toute cible : on annule (la carte reste à sa position serveur).
      return
    }
    switch (hit.target) {
      case TARGETS.TABLE:
        // Pose/replace sur le tapis à la position de relâchement.
        dispatch('move', { cardId, x: hit.x, y: hit.y })
        break
      case TARGETS.SABOT:
        dispatch('transfer', { cardId, target: TARGETS.SABOT })
        break
      case TARGETS.AVATAR:
        // Don de carte : elle passe dans la main privée du joueur cible.
        dispatch('transfer', { cardId, target: TARGETS.AVATAR, ownerId: hit.ownerId })
        break
      case TARGETS.HAND:
        // Vers ma propre main (zone main basse).
        dispatch('transfer', { cardId, target: TARGETS.HAND, ownerId: myUserId })
        break
    }
  }

  // ---- Handlers des cartes de table ----
  function onCardDrag(e) {
    const { cardId, x, y } = e.detail
    // Position live locale + diffusion aux autres clients (fluidité).
    dispatch('drag', { cardId, x, y })
  }
  function onCardDrop(e) {
    const { cardId, clientX, clientY } = e.detail
    resolveDrop(clientX, clientY, cardId, 'table')
  }
  function onCardFlip(e) { dispatch('flip', e.detail) }
  function onCardFront(e) { dispatch('front', e.detail) }
  function onCardRotate(e) { dispatch('rotate', e.detail) }

  // ---- Handlers du sabot ----
  function onSabotDraw(e) {
    const { clientX, clientY } = e.detail
    refreshRect()
    const hit = dropAt(clientX, clientY, { tableRect })
    if (!hit) return
    switch (hit.target) {
      case TARGETS.TABLE:
        dispatch('sabotDraw', { target: TARGETS.TABLE, x: hit.x, y: hit.y })
        break
      case TARGETS.AVATAR:
        dispatch('sabotDraw', { target: TARGETS.AVATAR, ownerId: hit.ownerId })
        break
      case TARGETS.HAND:
        dispatch('sabotDraw', { target: TARGETS.HAND, ownerId: myUserId })
        break
      // drop sur sabot lui-même : no-op.
    }
  }

  // ---- Position réelle d'une carte de table (prise en compte du drag live) ----
  function cardPos(card) {
    if (liveCardId === card.id) return { x: liveX, y: liveY }
    return { x: card.x, y: card.y }
  }
</script>

<div class="table-scroll">
  <div
    bind:this={tableEl}
    class="table"
    data-drop="table"
  >
    <!-- tapis décoratif -->
    <div class="felt"></div>

    {#if !initialized}
      <div class="empty-hint">
        En attente d'initialisation du sabot…
        <small>(un joueur doit préparer le jeu)</small>
      </div>
    {/if}

    <!-- Sabot -->
    <Sabot
      count={sabotCount}
      x={SABOT_POS.x}
      y={SABOT_POS.y}
      on:draw={onSabotDraw}
    />

    <!-- Avatars des joueurs connectés -->
    {#each players as p (p.userId)}
      <Avatar player={p} isMe={p.userId === myUserId} />
    {/each}

    <!-- Cartes sur la table -->
    {#each table as card (card.id)}
      <div class="card-anchor" style="left:{cardPos(card).x}px; top:{cardPos(card).y}px; z-index:{card.z || 1};">
        <Card
          c={card}
          zone="table"
          on:drag={onCardDrag}
          on:drop={onCardDrop}
          on:flip={onCardFlip}
          on:front={onCardFront}
          on:rotate={onCardRotate}
        />
      </div>
    {/each}
  </div>
</div>

<style>
  .table-scroll {
    flex: 1;
    overflow: auto;
    position: relative;
    background: #062016;
  }
  .table {
    position: relative;
    width: 1000px;
    min-height: 640px;
    margin: 0 auto;
  }
  .felt {
    position: absolute;
    inset: 0;
    background: radial-gradient(circle at 50% 45%, #1f7a52 0%, #135a3c 55%, #0a3a26 100%);
    box-shadow: inset 0 0 120px rgba(0,0,0,0.5);
  }
  .empty-hint {
    position: absolute;
    top: 50%; left: 50%;
    transform: translate(-50%, -50%);
    color: rgba(255,255,255,0.75);
    font-family: system-ui, sans-serif;
    text-align: center;
  }
  .empty-hint small { display: block; opacity: .7; margin-top: 4px; }
  .card-anchor {
    position: absolute;
    transform: translate(-50%, -50%);
    /* Les cartes sont positionnées par leur centre (le serveur stocke x/y du
       coin sup-gauche de l'ancêtre .card-slot interne, mais on translate ici
       pour coller au ressenti "je saisis la carte par son centre"). */
  }
</style>
