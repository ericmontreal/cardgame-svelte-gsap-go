<script>
  // Zone main joueur (§7) : bande horizontale basse contenant les cartes
  // privées du joueur courant. Uniquement visibles par le propriétaire.
  // On peut y déposer une carte depuis la table (don à soi-même) ou y faire
  // revenir une carte, et glisser une carte vers la table.
  import { createEventDispatcher } from 'svelte'
  import Card from './Card.svelte'
  import { dropAt, cardCornerOnTable, TARGETS } from './drag.js'
  import { HAND_CHROME } from './geometry.js'

  export let hand = []          // cartes privées du joueur
  export let myUserId = ''
  // Taille d'affichage des cartes : celle des cartes du tapis À L'ÉCRAN
  // (CARD_W/CARD_H × échelle du tapis), fournie par App.svelte. Les deux
  // tailles différaient : une carte changeait de taille en passant de la main
  // au tapis.
  export let cardW = 56
  export let cardH = 78

  const dispatch = createEventDispatcher()

  // La main est cible de drop : déposer une carte ici la met en main privée.
  let hovered = false
  function onDragEnter(e) { e.preventDefault(); hovered = true }
  function onDragLeave() { hovered = false }
  function onDragOver(e) { e.preventDefault() }

  // Drop sur la main (depuis la table). On laisse le hit-test global trancher
  // (data-drop="hand"), mais on gère aussi le cas d'un drag pointer-based qui
  // relâche directement au-dessus de la zone main.
  function onPointerUp(e) {
    // Si un drag de carte (pointer) se termine au-dessus de la main, le hit-test
    // global renverra 'hand' avec ownerId=self ; rien à faire ici en plus.
  }

  // Drag d'une carte de la main vers l'extérieur : le parent fait le hit-test.
  function onCardDrop(e) {
    const { cardId, clientX, clientY, rect } = e.detail
    const hit = dropAt(clientX, clientY)
    if (hit && (hit.target === TARGETS.AVATAR)) {
      dispatch('transfer', { cardId, target: TARGETS.AVATAR, ownerId: hit.ownerId })
    } else if (hit && hit.target === TARGETS.HAND) {
      // Lâchée dans sa propre main : la carte change de place dans la main.
      reorder(cardId, clientX)
    } else {
      // Par défaut, déposer hors de la main = poser sur la table, le coin de
      // la carte là où le joueur la voit au lâcher (et non sous le pointeur).
      const pos = (rect && cardCornerOnTable(rect.left, rect.top)) ?? { x: hit?.x ?? 0, y: hit?.y ?? 0 }
      dispatch('transfer', { cardId, target: TARGETS.TABLE, x: Math.round(pos.x), y: Math.round(pos.y) })
    }
  }
  // Réordonnancement : la carte s'insère avant la première autre carte dont le
  // centre est à droite du point de lâcher. Les positions sont lues à l'écran,
  // la carte saisie exclue (elle suit encore le pointeur).
  let fanEl
  function reorder(cardId, clientX) {
    const others = hand.filter((c) => c.id !== cardId)
    const slots = fanEl ? [...fanEl.querySelectorAll('.hand-card')] : []
    let index = others.length
    for (let i = 0, k = 0; i < hand.length; i++) {
      if (hand[i].id === cardId) continue
      const r = slots[i]?.getBoundingClientRect()
      if (r && clientX < r.left + r.width / 2) { index = k; break }
      k++
    }
    const moved = hand.find((c) => c.id === cardId)
    const next = [...others.slice(0, index), moved, ...others.slice(index)]
    if (next.every((c, i) => c.id === hand[i].id)) return // aucun changement
    dispatch('reorder', { cardIds: next.map((c) => c.id) })
  }
  function onCardFlip(e) { dispatch('flip', e.detail) }
  function onCardFront(e) { dispatch('front', e.detail) }
</script>

<div
  class="hand"
  class:hovered
  data-drop="hand"
  data-owner={myUserId}
  style="min-height:{cardH + HAND_CHROME}px;"
  on:dragenter={onDragEnter}
  on:dragleave={onDragLeave}
  on:dragover={onDragOver}
  on:pointerup={onPointerUp}
>
  {#if hand.length === 0}
    <div class="empty">Votre main est vide. Déposez une carte ici pour la cacher aux autres.</div>
  {:else}
    <div class="fan" bind:this={fanEl}>
      {#each hand as card (card.id)}
        <div class="hand-card">
          <Card
            c={card}
            zone="hand"
            width={cardW}
            height={cardH}
            on:drop={onCardDrop}
            on:flip={onCardFlip}
            on:front={onCardFront}
          />
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  /* Hauteur = carte + HAND_CHROME (geometry.js : 2 x 6px de marge + 2px de
     filet), posée en ligne : elle suit la taille des cartes, donc l'échelle du
     tapis. Toute modification des marges ou du filet doit y être reportée,
     sinon le calcul d'échelle d'App.svelte se trompe de quelques pixels. */
  .hand {
    box-sizing: border-box;
    padding: 6px 14px;
    background: linear-gradient(180deg, rgba(0,0,0,0.35), rgba(0,0,0,0.6));
    border-top: 2px solid rgba(255,255,255,0.1);
    display: flex;
    align-items: center;
    overflow-x: auto;
    transition: background .15s ease;
  }
  /* Même chose dans l'autre sens : une carte tirée de la main vers le tapis
     ne doit pas être rognée par le défilement horizontal de la main. */
  :global(.hand:has(.card.dragging)) { overflow: visible; }
  .hand.hovered { background: linear-gradient(180deg, rgba(47,158,99,0.25), rgba(0,0,0,0.6)); }
  .empty {
    color: rgba(255,255,255,0.55);
    font-family: system-ui, sans-serif;
    font-size: .9rem;
    width: 100%;
    text-align: center;
  }
  /* Cartes juxtaposées, bord à bord, sans intervalle. */
  .fan { display: flex; gap: 0; align-items: flex-end; }
  .hand-card { position: relative; }
</style>
