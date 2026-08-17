<script>
  // Sabot (shoe) : pile de cartes face cachée, posée sur le tapis (§11).
  //   - draggable : "glisser le sabot" tire la carte du sommet vers la cible (§6).
  //   - drop target : y déposer une carte la renvoie dans le sabot.
  import { createEventDispatcher } from 'svelte'
  import { gsap } from 'gsap'

  export let count = 0           // nombre de cartes dans le sabot
  export let x = 40
  export let y = 40
  export let cardW = 92
  export let cardH = 128

  const dispatch = createEventDispatcher()

  let dragging = false
  let startX = 0, startY = 0
  let moved = false

  // ---- Fantôme de tirage ----------------------------------------------------
  // Sans lui, glisser le sabot ne montrait RIEN : la carte n'apparaissait qu'au
  // relâchement, une fois l'état serveur revenu. Le geste était identique à
  // celui d'une carte du tapis (qui, elle, suit le curseur via Card.svelte) mais
  // sans aucun retour visuel — on ne savait pas si on tirait quelque chose.
  //
  // Le fantôme est affiché DOS VISIBLE, et ce n'est pas un pis-aller : le client
  // ignore quelle carte il tire. C'est le serveur qui prélève le sommet du sabot
  // au relâchement (sabotDraw envoie un cardId vide, cf. handlers.go). Montrer
  // une face ici obligerait à deviner, donc à mentir une fois sur deux.
  //
  // position:fixed, car les cibles de dépôt (avatars, main du joueur) débordent
  // du tapis : un fantôme positionné dans le repère de la table serait tronqué
  // dès qu'on sort du feutre. Aucun ancêtre ne porte de transform, le repère
  // reste donc bien celui de la fenêtre.
  let ghostEl = null
  let ghostVisible = false
  let grabDX = 0, grabDY = 0     // point de prise, relatif au coin du sabot

  function moveGhost(clientX, clientY) {
    if (!ghostEl) return
    gsap.set(ghostEl, { x: clientX - grabDX, y: clientY - grabDY })
  }

  function onPointerDown(e) {
    if (count <= 0) return
    dragging = true
    moved = false
    startX = e.clientX
    startY = e.clientY
    // Décalage de prise : la carte se détache exactement là où on l'a saisie,
    // au lieu de recentrer son coin sous le curseur (même parti pris que
    // Card.svelte, qui transmet curX/curY plutôt que la position du pointeur).
    const r = e.currentTarget.getBoundingClientRect()
    grabDX = e.clientX - r.left
    grabDY = e.clientY - r.top
    moveGhost(e.clientX, e.clientY)
    e.currentTarget.setPointerCapture(e.pointerId)
    e.preventDefault()
  }
  function onPointerMove(e) {
    if (!dragging) return
    if (!moved && Math.hypot(e.clientX - startX, e.clientY - startY) > 4) moved = true
    if (!moved) return
    // Le fantôme n'apparaît qu'au franchissement du seuil : un simple clic sur
    // le sabot ne doit pas faire clignoter une carte.
    ghostVisible = true
    moveGhost(e.clientX, e.clientY)
  }
  function onPointerUp(e) {
    if (!dragging) return
    dragging = false
    try { e.currentTarget.releasePointerCapture(e.pointerId) } catch {}
    // Rect du fantôme mesuré AVANT de le masquer : c'est sa position réellement
    // affichée à l'écran, donc la seule qui garantisse que la carte se pose là
    // où le joueur la voit. Le mesurer plutôt que le recalculer met ce code à
    // l'abri d'un changement de taille ou de mise en page du fantôme.
    const r = ghostEl ? ghostEl.getBoundingClientRect() : null
    ghostVisible = false
    if (moved) {
      // Drag du sabot terminé : on tire la carte du sommet vers la cible sous
      // le curseur. Le parent fait le hit-test et envoie 'sabotDraw'.
      //
      // On transmet aussi le coin sup-gauche du fantôme, en coordonnées écran.
      // Le pointeur seul ne suffit pas à poser la carte là où on la voit : il
      // ignore le décalage de prise, et la carte se recentrerait sous le
      // curseur. C'est la même correction que 5f9616d, qui n'avait porté que
      // sur les cartes du tapis — le chemin du sabot était resté en dehors,
      // invisible tant qu'aucun fantôme ne montrait la position réelle.
      dispatch('draw', {
        clientX: e.clientX,
        clientY: e.clientY,
        ghostX: r ? r.left : e.clientX - grabDX,
        ghostY: r ? r.top : e.clientY - grabDY,
      })
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

<!-- Fantôme de tirage. Toujours présent dans le DOM plutôt que monté à la
     volée : gsap.set doit pouvoir le positionner dès le premier pointermove,
     sans attendre un cycle de rendu de Svelte — sinon la carte apparaîtrait
     une frame au coin de l'écran avant de rejoindre le curseur. -->
<svg
  bind:this={ghostEl}
  class="ghost"
  class:visible={ghostVisible}
  width={cardW}
  height={cardH}
  viewBox="0 0 200 280"
  preserveAspectRatio="xMidYMid meet"
  aria-hidden="true"
>
  <use href="#sym-back" xlink:href="#sym-back"></use>
</svg>

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
  .ghost {
    position: fixed;
    left: 0; top: 0;
    border-radius: 8px;
    background: #fff;
    /* Même relief qu'une carte de tapis en cours de glisser (.card.dragging). */
    box-shadow: 0 10px 24px rgba(0,0,0,0.5);
    /* Impératif : le fantôme est sous le curseur au relâchement, et dropAt()
       interroge document.elementsFromPoint pour trouver la cible. Sans ceci il
       s'interposerait devant l'avatar ou le sabot visés. */
    pointer-events: none;
    z-index: 9999;
    opacity: 0;
    visibility: hidden;
  }
  .ghost.visible { opacity: 1; visibility: visible; }
</style>
