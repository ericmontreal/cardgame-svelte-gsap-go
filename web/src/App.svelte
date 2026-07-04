<script>
  import { onMount, tick } from 'svelte'
  import Card from './lib/Card.svelte'
  import { ensureInlineSprite } from './lib/svg-sprite'
  import { dealToHands, fullDeck } from './lib/deck'
  import { dealFromCenter } from './lib/cards-anim'
  import { createWsClient } from './lib/ws-client'

  const PLAYERS = 4
  const PER_PLAYER = 5

  let spriteReady = false
  // Mains vides tant qu'aucun deal n'a été synchronisé
  let allHands = Array.from({ length: PLAYERS }, () => [])

  // Références DOM / composants collectées via bindings Svelte (pas de
  // querySelectorAll global) :
  //  - handEls : conteneurs de chaque main (cibles de l'animation)
  //  - cardRefs : instances Card, pour récupérer leur nœud via getEl()
  let deckEl
  let handEls = []
  let cardRefs = []

  // État connexion WS (affiché à l'utilisateur)
  let connStatus = 'idle'
  let connInfo = ''

  let ws
  let dealBroadcastTimer = null
  let lastDealSeq = null

  // Réinitialise les tableaux de références avant un nouveau rendu des cartes.
  function resetRefs() {
    cardRefs = new Array(PLAYERS * PER_PLAYER)
    handEls = new Array(PLAYERS)
  }

  // Retourne les nœuds DOM des cartes présentes, dans l'ordre, via les
  // instances Card (méthode getEl exposée par le composant).
  function collectCardNodes() {
    return (cardRefs || [])
      .filter(Boolean)
      .map((c) => c.getEl?.())
      .filter(Boolean)
  }

  // Bascule l'état d'une carte. On réassigne l'objet dans la main pour la
  // réactivité Svelte (flux unidirectionnel : le parent est seul maître de
  // l'état, l'enfant émet 'flip' sans toucher à sa prop).
  function flipCard(h, i) {
    const hand = allHands[h]
    if (!hand || !hand[i]) return
    const card = hand[i]
    allHands[h][i] = { ...card, faceUp: !card.faceUp }
  }

  // Applique un jeu de mains reçu (du serveur ou généré localement), puis
  // lance l'animation GSAP de distribution depuis le centre.
  async function applyHands(hands) {
    resetRefs()
    allHands = hands
    await tick()
    if (!spriteReady) return
    const cardNodes = collectCardNodes()
    const targets = handEls.filter(Boolean)
    if (cardNodes.length && targets.length) {
      dealFromCenter(cardNodes, deckEl, targets, { perPlayer: PER_PLAYER, stagger: 0.06 })
    }
  }

  onMount(async () => {
    await ensureInlineSprite('/cards.svg')
    spriteReady = true

    ws = createWsClient({ room: 'lobby' })

    ws.onStatus((status, extra) => {
      connStatus = status
      connInfo = extra && extra.attempt ? ` (essai ${extra.attempt})` : ''
    })

    // À la réception d'un deal diffusé par un autre client, on l'applique
    // sans régénérer de distribution (synchro coopérative).
    ws.on((msg) => {
      if (msg.type === 'deal' && msg.payload && Array.isArray(msg.payload.hands)) {
        // Anti-doublon : on ignore strictement le même jeu de mains deux fois.
        const seq = JSON.stringify(msg.payload.hands)
        if (seq === lastDealSeq) return
        lastDealSeq = seq
        if (dealBroadcastTimer) { clearTimeout(dealBroadcastTimer); dealBroadcastTimer = null }
        applyHands(msg.payload.hands)
      }
    })

    ws.connect()

    // Stratégie de synchro coopérative : un client qui vient d'arriver attend
    // brièvement qu'un autre lui envoie un deal. Si rien n'arrive, il génère
    // lui-même la distribution et la diffuse à la room.
    dealBroadcastTimer = setTimeout(() => {
      if (lastDealSeq) return // un deal a déjà été reçu
      const hands = dealToHands(fullDeck(), PLAYERS, PER_PLAYER)
      lastDealSeq = JSON.stringify(hands)
      ws.sendDeal(hands, PER_PLAYER)
      applyHands(hands)
    }, 1200)
  })
</script>

<style>
  .table { position: relative; min-height: 70vh; }
  .center { position:absolute; left:50%; top:50%; transform:translate(-50%,-50%); }
  .row    { display:flex; gap:12px; flex-wrap:wrap; justify-content:center; margin:12px 0; }
  .status {
    display:inline-block; padding:4px 10px; border-radius:999px;
    font-size:13px; margin:0 0 12px;
    background:#eef; color:#335; border:1px solid #ccd;
  }
  .status.open        { background:#e6f6e6; color:#225; border-color:#9d9; }
  .status.connecting  { background:#fff8e0; color:#553; border-color:#dc9; }
  .status.reconnecting{ background:#fff0e0; color:#642; border-color:#ea7; }
  .status.error       { background:#fde; color:#622; border-color:#e99; }
</style>

<main style="padding:16px; font-family:system-ui,Segoe UI,Roboto,Helvetica,Arial">
  <h1 style="margin:0 0 8px">Jeu de cartes — Svelte + GSAP</h1>
  <p style="margin:0 0 16px">Distribution depuis la pile centrale vers 4 mains.</p>

  <div class="status {connStatus}">
    Connexion : <strong>{connStatus}{connInfo}</strong>
  </div>

  <div class="table">
    <!-- Pile centrale -->
    <div class="center" bind:this={deckEl}>
      {#if spriteReady}
        <svg width="40" height="56" viewBox="0 0 200 280"><use href="#sym-back"/></svg>
      {/if}
    </div>

    {#if spriteReady}
      <!-- Les 4 mains, rendues de façon pilotée par les données -->
      {#each allHands as hand, h}
        <div class="row" bind:this={handEls[h]}>
          {#each hand as c, i}
            <Card
              bind:this={cardRefs[h * PER_PLAYER + i]}
              {c}
              width={100}
              height={140}
              on:flip={() => flipCard(h, i)}
            />
          {/each}
        </div>
      {/each}
    {/if}
  </div>
</main>
