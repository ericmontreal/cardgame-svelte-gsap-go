<script>
  // Zone main joueur (§7) : bande horizontale basse contenant les cartes
  // privées du joueur courant. Uniquement visibles par le propriétaire.
  // On peut y déposer une carte depuis la table (don à soi-même) ou y faire
  // revenir une carte, et glisser une carte vers la table.
  import { createEventDispatcher } from 'svelte'
  import Card from './Card.svelte'
  import { dropAt, TARGETS } from './drag.js'

  export let hand = []          // cartes privées du joueur
  export let myUserId = ''

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
    const { cardId, clientX, clientY } = e.detail
    const hit = dropAt(clientX, clientY)
    if (hit && (hit.target === TARGETS.AVATAR)) {
      dispatch('transfer', { cardId, target: TARGETS.AVATAR, ownerId: hit.ownerId })
    } else if (hit && hit.target === TARGETS.HAND) {
      // rester dans ma propre main -> pas de changement de zone.
    } else {
      // Par défaut, déposer hors de la main = poser sur la table.
      dispatch('transfer', { cardId, target: TARGETS.TABLE, x: hit?.x ?? 0, y: hit?.y ?? 0 })
    }
  }
  function onCardFlip(e) { dispatch('flip', e.detail) }
  function onCardFront(e) { dispatch('front', e.detail) }
</script>

<div
  class="hand"
  class:hovered
  data-drop="hand"
  data-owner={myUserId}
  on:dragenter={onDragEnter}
  on:dragleave={onDragLeave}
  on:dragover={onDragOver}
  on:pointerup={onPointerUp}
>
  {#if hand.length === 0}
    <div class="empty">Votre main est vide. Déposez une carte ici pour la cacher aux autres.</div>
  {:else}
    <div class="fan">
      {#each hand as card (card.id)}
        <div class="hand-card">
          <Card
            c={card}
            zone="hand"
            width={72}
            height={100}
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
  .hand {
    min-height: 130px;
    padding: 10px 14px;
    background: linear-gradient(180deg, rgba(0,0,0,0.35), rgba(0,0,0,0.6));
    border-top: 2px solid rgba(255,255,255,0.1);
    display: flex;
    align-items: center;
    overflow-x: auto;
    transition: background .15s ease;
  }
  .hand.hovered { background: linear-gradient(180deg, rgba(47,158,99,0.25), rgba(0,0,0,0.6)); }
  .empty {
    color: rgba(255,255,255,0.55);
    font-family: system-ui, sans-serif;
    font-size: .9rem;
    width: 100%;
    text-align: center;
  }
  .fan { display: flex; gap: 8px; align-items: flex-end; }
  .hand-card { position: relative; }
</style>
