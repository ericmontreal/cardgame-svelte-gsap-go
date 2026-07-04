<script>
  // Sabot (shoe) : pile de cartes face cachée, posée sur le tapis (§11).
  //   - draggable : "glisser le sabot" tire la carte du sommet vers la cible (§6).
  //   - drop target : y déposer une carte la renvoie dans le sabot.
  import { createEventDispatcher } from 'svelte'

  export let count = 0           // nombre de cartes dans le sabot
  export let x = 40
  export let y = 40
  export let cardW = 92
  export let cardH = 128

  const dispatch = createEventDispatcher()

  let dragging = false
  let startX = 0, startY = 0
  let moved = false

  function onPointerDown(e) {
    if (count <= 0) return
    dragging = true
    moved = false
    startX = e.clientX
    startY = e.clientY
    e.currentTarget.setPointerCapture(e.pointerId)
    e.preventDefault()
  }
  function onPointerMove(e) {
    if (!dragging) return
    if (!moved && Math.hypot(e.clientX - startX, e.clientY - startY) > 4) moved = true
    // Pendant le drag du sabot, on peut afficher un ghost (optionnel) : ici on
    // se contente de marquer le mouvement pour déclencher le tirage au drop.
  }
  function onPointerUp(e) {
    if (!dragging) return
    dragging = false
    try { e.currentTarget.releasePointerCapture(e.pointerId) } catch {}
    if (moved) {
      // Drag du sabot terminé : on tire la carte du sommet vers la cible sous
      // le curseur. Le parent fait le hit-test et envoie 'sabotDraw'.
      dispatch('draw', { clientX: e.clientX, clientY: e.clientY })
    }
  }

  // Le sabot est aussi cible de drop (remise de carte).
  let hovered = false
  function onDragEnter(e) { e.preventDefault(); hovered = true }
  function onDragLeave() { hovered = false }
  function onDragOver(e) { e.preventDefault() }
</script>

<div
  class="sabot"
  class:empty={count === 0}
  class:hovered
  class:dragging
  data-drop="sabot"
  style="left:{x}px; top:{y}px; --cw:{cardW}px; --ch:{cardH}px;"
  title="Sabot ({count} carte{count > 1 ? 's' : ''}) — glissez pour tirer"
  on:pointerdown={onPointerDown}
  on:pointermove={onPointerMove}
  on:pointerup={onPointerUp}
  on:pointercancel={onPointerUp}
  on:dragenter={onDragEnter}
  on:dragleave={onDragLeave}
  on:dragover={onDragOver}
>
  {#if count > 0}
    <!-- Empilement visuel : quelques dos décalés pour l'effet "pile" -->
    {#each [0, 1, 2, 3] as i}
      {#if i < Math.min(count, 4)}
        <svg
          class="stack-card"
          class:top={i === Math.min(count, 4) - 1}
          width={cardW}
          height={cardH}
          viewBox="0 0 200 280"
          preserveAspectRatio="xMidYMid meet"
          style="--i:{i}"
        >
          <use href="#sym-back" xlink:href="#sym-back"></use>
        </svg>
      {/if}
    {/each}
  {/if}
  <div class="badge">{count}</div>
  <div class="label">Sabot</div>
</div>

<style>
  .sabot {
    position: absolute;
    width: var(--cw);
    height: var(--ch);
    cursor: grab;
    touch-action: none;
    user-select: none;
  }
  .sabot.empty { cursor: default; opacity: .55; }
  .sabot.dragging { cursor: grabbing; }
  .stack-card {
    position: absolute;
    left: 0; top: 0;
    border-radius: 8px;
    box-shadow: 0 2px 5px rgba(0,0,0,0.3);
    background: #fff;
    transform: translate(calc(var(--i) * -2px), calc(var(--i) * -2px));
  }
  .stack-card.top { box-shadow: 0 6px 14px rgba(0,0,0,0.45); }
  .badge {
    position: absolute;
    top: -10px; right: -10px;
    background: #2f9e63;
    color: #fff;
    font-size: 12px;
    font-weight: 700;
    min-width: 22px;
    height: 22px;
    padding: 0 6px;
    border-radius: 999px;
    display: grid;
    place-items: center;
    font-family: system-ui, sans-serif;
    box-shadow: 0 2px 6px rgba(0,0,0,0.4);
  }
  .label {
    position: absolute;
    bottom: -22px; left: 50%;
    transform: translateX(-50%);
    background: rgba(0,0,0,0.55);
    color: #fff;
    font-size: 11px;
    padding: 1px 8px;
    border-radius: 999px;
    font-family: system-ui, sans-serif;
    white-space: nowrap;
  }
  .sabot.hovered .stack-card.top {
    box-shadow: 0 0 0 4px rgba(255,210,122,0.5), 0 6px 14px rgba(0,0,0,0.5);
  }
</style>
