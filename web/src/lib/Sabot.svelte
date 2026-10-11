<script>
  // Sabot (shoe) : pile de cartes face cachée, posée sur le tapis (§11).
  //   - draggable : "glisser le sabot" tire la carte du sommet vers la cible (§6).
  //   - drop target : y déposer une carte la renvoie dans le sabot.
  //   - bouton « Mélanger » : bat les cartes restées dans le sabot.
  import { createEventDispatcher, onMount } from 'svelte'
  import { gsap } from 'gsap'

  export let count = 0           // nombre de cartes dans le sabot
  export let x = 40
  export let y = 40
  export let cardW = 92
  export let cardH = 128
  // Horodatage du dernier mélange annoncé par le serveur (store sabotShuffle).
  // Chaque nouvelle valeur rejoue l'animation, chez tous les joueurs.
  export let shuffleAt = 0
  // Échelle d'affichage du tapis (Table.svelte). Le fantôme vit hors de la
  // table réduite : il est dimensionné à la main pour garder la taille des
  // cartes que le joueur voit.
  export let scale = 1

  const dispatch = createEventDispatcher()

  // ---- Mélange ----------------------------------------------------------------
  // Le clic n'anime rien par lui-même : il demande le mélange au serveur, et
  // c'est l'annonce 'sabotShuffled' en retour qui fait battre les cartes. Le
  // joueur qui clique voit donc exactement ce que voient les autres, et rien
  // si le serveur a refusé (sabot de moins de deux cartes).
  let stackEls = []
  // Valeur présente au montage : un mélange antérieur (avant un passage par le
  // menu « Nouvelle partie », par ex.) ne doit pas être rejoué au retour.
  let lastShuffleAt = shuffleAt
  $: if (shuffleAt && shuffleAt !== lastShuffleAt) {
    lastShuffleAt = shuffleAt
    playShuffle()
  }

  function requestShuffle() {
    if (count < 2) return
    dispatch('shuffle')
  }

  function playShuffle() {
    const els = stackEls.filter(Boolean)
    if (els.length === 0) return
    if (window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return
    gsap.killTweensOf(els)
    // Les dos empilés s'écartent en deux paquets puis se réentrelacent, deux
    // fois : un battage « à la queue d'aronde » stylisé. À la fin, on rend la
    // main au CSS (clearProps), qui porte le décalage de l'effet pile.
    const tl = gsap.timeline({ onComplete: () => gsap.set(els, { clearProps: 'transform' }) })
    for (let k = 0; k < 2; k++) {
      tl.to(els, {
        x: (i) => (i % 2 ? 1 : -1) * cardW * 0.45,
        y: (i) => -2 * i,
        rotation: (i) => (i % 2 ? 7 : -7),
        duration: 0.16,
        ease: 'power2.out',
      }).to(els, {
        x: (i) => -2 * i,
        y: (i) => -2 * i,
        rotation: 0,
        duration: 0.16,
        ease: 'power2.in',
        stagger: 0.03,
      })
    }
  }

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
  // dès qu'on sort du feutre. Il est en outre DÉPLACÉ sous <body> au montage :
  // la table porte désormais un transform (mise à l'échelle), et un ancêtre
  // transformé capture les éléments fixes — le fantôme aurait suivi le repère
  // réduit de la table et été rogné par sa zone de défilement.
  let ghostEl = null
  onMount(() => {
    // Svelte le détachera de là où il se trouve à la destruction du composant.
    if (ghostEl) document.body.appendChild(ghostEl)
  })
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
  <!-- Emplacement du sabot, creusé dans le feutre : il marque sa place même
       quand le sabot est vide. -->
  <div class="well" aria-hidden="true"></div>
  {#if count > 0}
    <!-- Empilement visuel : quelques dos décalés pour l'effet "pile" -->
    {#each [0, 1, 2, 3] as i}
      {#if i < Math.min(count, 4)}
        <svg
          bind:this={stackEls[i]}
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
  <!-- pointerdown arrêté ici : sans cela, le clic remonterait au sabot (début
       de tirage, capture du pointeur) puis au tapis (rectangle de sélection). -->
  <!-- Contre-échelle : le bouton garde sa taille réelle sur tablette, où le
       tapis réduit le rendrait trop petit pour un doigt. -->
  <button
    class="shuffle"
    type="button"
    style="transform: translateX(-50%) scale({1 / scale});"
    disabled={count < 2}
    title={count < 2 ? 'Pas assez de cartes à mélanger' : 'Mélanger les cartes du sabot (le tapis et les mains ne sont pas ramassés)'}
    on:pointerdown|stopPropagation
    on:click|stopPropagation={requestShuffle}
  >Mélanger</button>
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
  style="width:{cardW * scale}px; height:{cardH * scale}px;"
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
  .sabot.empty { cursor: default; }
  /* Sabot vide : tout s'estompe, sauf son emplacement, qui reste marqué. */
  .sabot.empty > :not(.well) { opacity: .55; }
  /* Effet de creux : un feutre un peu plus sombre, ombré par le haut comme un
     logement taillé dans la table, avec un léger reflet sur le bord bas. */
  .well {
    position: absolute;
    inset: -18px; /* ~10px à l'écran d'une tablette (échelle ~0,57) */
    border-radius: 16px;
    background: rgba(0, 0, 0, 0.22);
    box-shadow:
      inset 0 5px 12px rgba(0, 0, 0, 0.5),
      inset 0 -1px 0 rgba(255, 255, 255, 0.08);
    pointer-events: none;
  }
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
  .shuffle {
    position: absolute;
    top: calc(100% + 30px); left: 50%;
    transform: translateX(-50%); /* + contre-échelle, posée en ligne */
    transform-origin: top center;
    border: 1px solid rgba(255, 210, 122, 0.4);
    background: rgba(0, 0, 0, 0.55);
    color: #ffd27a;
    font-size: 11px;
    font-family: system-ui, sans-serif;
    padding: 2px 10px;
    border-radius: 999px;
    cursor: pointer;
    white-space: nowrap;
  }
  .shuffle:hover:not(:disabled) { background: rgba(255, 210, 122, 0.22); }
  .shuffle:disabled { opacity: .45; cursor: default; }
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
