<script>
  // Carte manipulable (§6) :
  //   - drag & drop libre (pointer events) avec animations GSAP
  //   - double-clic = retournement (flip)
  //   - clic simple (sans déplacement) = mise au premier plan (Z)
  //   - rotation possible (molette + Ctrl, ou bouton dédié côté parent)
  //
  // La carte n'est qu'une vue : elle émet des événements vers le parent, qui
  // les transmet au serveur autoritaire. Aucune logique de jeu ici.
  import { createEventDispatcher, onMount, afterUpdate } from 'svelte'
  import { gsap } from 'gsap'

  // `c` : carte (état serveur). `zone` : 'table' | 'hand' (comportement drop).
  export let c = { id: '', faceId: '1_club', faceUp: true, x: 0, y: 0, z: 0, rotate: 0 }
  export let zone = 'table'
  export let width = 92
  export let height = 128
  // La carte est-elle manipulable par drag ? (les cartes du sabot ne le sont pas)
  export let draggable = true

  const dispatch = createEventDispatcher()

  let useEl          // <use> interne (rendu du symbole)
  let backId = 'back'
  let currentId = c.faceUp ? c.faceId : backId
  let rootEl        // élément racine pour les animations GSAP

  // ---- Rendu du symbole via <use href="#sym-..."> ----
  function setBoth() {
    if (!useEl) return
    const target = '#sym-' + (c.faceUp ? currentId : backId)
    useEl.setAttribute('href', target)
    useEl.setAttributeNS('http://www.w3.org/1999/xlink', 'xlink:href', target)
  }
  function setFace(id) { currentId = id; setBoth() }
  export { setFace }

  $: currentId = c.faceId
  $: faceUpChanged = c.faceUp

  // Animation de retournement 3D au changement de face (GSAP obligatoire §3).
  let lastFaceUp = c.faceUp
  afterUpdate(() => {
    setBoth()
    if (c.faceUp !== lastFaceUp && rootEl) {
      gsap.fromTo(
        rootEl,
        { rotateY: lastFaceUp ? 0 : 180 },
        { rotateY: c.faceUp ? 0 : 180, duration: 0.35, ease: 'power2.out' }
      )
      lastFaceUp = c.faceUp
    }
  })

  onMount(() => {
    const symRoot = document.getElementById('__cards_symbols__')
    if (symRoot) {
      const hit = Array.from(symRoot.querySelectorAll('symbol'))
        .map((s) => s.id)
        .find((id) => /back/i.test(id))
      if (hit) backId = hit.replace(/^sym-/, '')
    }
    setBoth()
  })

  // ---- Drag & drop par pointer events ----
  let dragging = false
  let moved = false
  let startX = 0, startY = 0      // position initiale du pointeur
  let originX = 0, originY = 0    // position initiale de la carte (transform)

  function onPointerDown(e) {
    if (!draggable) return
    // Un seul pointeur à la fois.
    if (dragging) return
    // Bouton gauche uniquement (on:click gérera les autres cas).
    if (e.button !== 0 && e.pointerType === 'mouse') return
    dragging = true
    moved = false
    startX = e.clientX
    startY = e.clientY
    originX = c.x
    originY = c.y
    // Capturer dès pointerdown (et pas seulement après le seuil de 3px, cf.
    // onPointerMove) : sinon un mouvement rapide/ample peut sortir le
    // curseur de la petite carte avant que la capture ne s'engage, et les
    // pointermove suivants ne lui parviennent plus jamais (drag cassé). Ceci
    // n'affecte plus le clic/double-clic : ils sont détectés manuellement
    // dans onPointerUp plutôt que via les événements natifs click/dblclick,
    // peu fiables sur un <use> SVG référencé sous Chromium.
    e.currentTarget.setPointerCapture(e.pointerId)
    e.preventDefault()
    dispatch('dragstart', { cardId: c.id })
  }

  function onPointerMove(e) {
    if (!dragging) return
    const dx = e.clientX - startX
    const dy = e.clientY - startY
    if (!moved && Math.hypot(dx, dy) > 3) moved = true
    if (!moved) return
    const nx = originX + dx
    const ny = originY + dy
    // Position live du drag (pour fluidité inter-clients + vue locale).
    dispatch('drag', { cardId: c.id, x: nx, y: ny })
    if (rootEl) {
      // Le conteneur parent (Table.svelte `.card-anchor`) est déjà positionné
      // à (originX, originY) : n'appliquer ici que le delta parcouru depuis
      // le point de prise, sinon la carte "saute" (double décalage).
      gsap.set(rootEl, { x: dx, y: dy })
    }
  }

  // ---- Clic / double-clic (détection manuelle, cf. onPointerUp) ----
  // dbl-clic = flip (§6). clic simple sans drag = premier plan (§6). On ne
  // s'appuie PAS sur les événements natifs click/dblclick : sur Chromium, un
  // <use> SVG référencé via href ne les synthétise pas de façon fiable après
  // un pointerdown/pointerup (constaté y compris cibles down/up identiques),
  // ce qui cassait silencieusement le flip et la mise au premier plan.
  let lastClickAt = 0
  const DBLCLICK_MS = 350

  function onPointerUp(e) {
    if (!dragging) return
    dragging = false
    try { e.currentTarget.releasePointerCapture(e.pointerId) } catch {}
    // Remise à zéro du transform local : la position finale est portée par
    // (c.x, c.y) une fois l'état confirmé par le serveur reçu.
    if (rootEl) gsap.set(rootEl, { x: 0, y: 0 })
    if (!moved) {
      dispatch('dragend', { cardId: c.id, moved: false, clientX: e.clientX, clientY: e.clientY })
      const now = Date.now()
      if (now - lastClickAt < DBLCLICK_MS) {
        lastClickAt = 0
        dispatch('flip', { cardId: c.id })
      } else {
        lastClickAt = now
        if (zone === 'table') dispatch('front', { cardId: c.id })
      }
      return
    }
    dispatch('drop', { cardId: c.id, zone, clientX: e.clientX, clientY: e.clientY })
  }

  // ---- Accessibilité clavier (flip) ----
  function onKey(e) {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      dispatch('flip', { cardId: c.id })
    }
  }
</script>

<div
  class="card-slot"
  class:is-hand={zone === 'hand'}
  style="--w:{width}px; --h:{height}px; z-index:{c.z || 1};"
>
  <svg
    bind:this={rootEl}
    class="card"
    class:face-up={c.faceUp}
    class:face-down={!c.faceUp}
    class:dragging
    width={width}
    height={height}
    viewBox="0 0 200 280"
    preserveAspectRatio="xMidYMid meet"
    role="button"
    tabindex="0"
    aria-label="Carte {c.faceUp ? c.faceId : 'cachée'}"
    on:pointerdown={onPointerDown}
    on:pointermove={onPointerMove}
    on:pointerup={onPointerUp}
    on:pointercancel={onPointerUp}
    on:keydown={onKey}
  >
    <use bind:this={useEl}></use>
  </svg>
</div>

<style>
  .card-slot {
    position: absolute;
    width: var(--w);
    height: var(--h);
    cursor: grab;
    touch-action: none; /* indispensable pour pointer events sans scroll */
    will-change: transform;
  }
  .card-slot.is-hand { position: relative; cursor: grab; }
  .card {
    width: 100%; height: 100%;
    border-radius: 8px;
    box-shadow: 0 4px 10px rgba(0,0,0,0.35);
    background: #fff;
    transform-style: preserve-3d;
    will-change: transform;
    user-select: none;
  }
  .card.dragging { cursor: grabbing; z-index: 9999; box-shadow: 0 10px 24px rgba(0,0,0,0.5); }
  .card:focus-visible { outline: 3px solid #4caa7a; outline-offset: 2px; }
</style>
