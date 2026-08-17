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
  export let snapEnabled = true  // aimantage entre cartes proches (menu init)

  const dispatch = createEventDispatcher()

  let tableEl
  let tableRect = null
  // Position fixe du sabot sur le tapis (zone de design), centrée sur le
  // feutre (table 1160x800, carte de sabot 92x128 par défaut). Le joueur peut
  // le déplacer en le glissant, mais son emplacement par défaut est stable.
  const SABOT_POS = { x: 534, y: 336 }

  function refreshRect() {
    if (tableEl) tableRect = tableEl.getBoundingClientRect()
  }
  onMount(() => {
    refreshRect()
    window.addEventListener('resize', refreshRect)
    window.addEventListener('scroll', refreshRect, true)
    window.addEventListener('keydown', onGlobalKey)
    return () => {
      window.removeEventListener('resize', refreshRect)
      window.removeEventListener('scroll', refreshRect, true)
      window.removeEventListener('keydown', onGlobalKey)
    }
  })
  // Recalcule le rect quand la table change de taille (nouvelles cartes...).
  $: table, players, sabotCount, initialized, refreshRect()

  // ---- Positions live d'un drag distant (autre joueur, simple ou groupé) ----
  // Dictionnaire cardId -> {x,y}. L'auto-abonnement $liveDrag garantit le
  // désabonnement à la destruction du composant et la réactivité du rendu.
  $: livePos = $liveDrag ?? {}

  // ---- Sélection multiple (état purement local, jamais envoyé au serveur) ---
  // Rectangle de sélection tracé sur le feutre + shift+clic pour ajouter ou
  // retirer une carte. Les manipulations groupées partent ensuite en une seule
  // mutation par lot (moveMany/flipMany) vers le serveur autoritaire.
  let selected = new Set()    // ids des cartes de table sélectionnées
  let marquee = null          // { x0,y0,x1,y1 } coords tapis, pendant le tracé
  let marqueeBase = new Set() // sélection au début du tracé (shift = union)

  // Toute carte qui quitte la table (transfert, nouveau sabot...) sort de la
  // sélection : l'état serveur fait foi.
  $: pruneSelection(table)
  function pruneSelection(tbl) {
    if (selected.size === 0) return
    const ids = new Set(tbl.map((c) => c.id))
    const next = new Set()
    for (const id of selected) if (ids.has(id)) next.add(id)
    if (next.size !== selected.size) selected = next
  }

  function onToggleSelect(e) {
    const id = e.detail.cardId
    const next = new Set(selected)
    if (next.has(id)) next.delete(id)
    else next.add(id)
    selected = next
  }

  // Dimensions par défaut d'une carte de table (Table.svelte ne passe pas de
  // width/height à <Card>). Une carte est repérée par son COIN supérieur
  // gauche et occupe [x, x+CARD_W] x [y, y+CARD_H] — cf. l'avertissement sur
  // `.card-anchor` dans les styles. Utilisées par l'aimantage, la détection de
  // pile et les actions groupées.
  const CARD_W = 92
  const CARD_H = 128

  // ---- Sélection de pile (Ctrl+clic) ----------------------------------------
  // Il n'existe pas de notion de pile côté serveur : une pile est un empilement
  // visuel émergent. On la reconstruit géométriquement, de proche en proche, à
  // partir de la carte cliquée : toute carte dont le rectangle chevauche
  // suffisamment celui d'un membre déjà retenu rejoint la pile (composante
  // connexe). La rotation est ignorée (approximation par rectangles droits).
  const PILE_OVERLAP_MIN = 0.6 // fraction de surface commune minimale

  function pileAt(cardId) {
    const start = table.find((c) => c.id === cardId)
    if (!start) return null
    const members = new Set([cardId])
    const queue = [start]
    while (queue.length) {
      const cur = queue.pop()
      for (const other of table) {
        if (members.has(other.id)) continue
        const wOv = Math.max(0, CARD_W - Math.abs(cur.x - other.x))
        const hOv = Math.max(0, CARD_H - Math.abs(cur.y - other.y))
        if ((wOv * hOv) / (CARD_W * CARD_H) >= PILE_OVERLAP_MIN) {
          members.add(other.id)
          queue.push(other)
        }
      }
    }
    return members
  }

  function onSelectPile(e) {
    const pile = pileAt(e.detail.cardId)
    if (!pile) return
    selected = e.detail.additive ? new Set([...selected, ...pile]) : pile
  }

  // ---- Actions groupées : empiler / étaler ------------------------------------
  // Marges de placement : une carte posée par ces actions doit rester entière
  // sur le feutre, dont le pourtour bois occupe 80px (`.felt` est en inset:80px,
  // cf. les styles plus bas).
  //
  // Ces bornes valent pour un COIN supérieur gauche, pas pour un centre : une
  // carte occupe [x, x+CARD_W] x [y, y+CARD_H]. Elles étaient auparavant
  // écrites `80 + CARD_W/2`, ce qui n'a de sens que pour des centres — la borne
  // haute laissait donc dépasser une demi-carte sur le bois, et la borne basse
  // gardait inutilement une demi-carte de jeu. Voir l'avertissement sur
  // `.card-anchor` dans les styles.
  const TABLE_W = 1160
  const TABLE_H = 800
  const FELT_INSET = 80
  const MIN_X = FELT_INSET
  const MIN_Y = FELT_INSET
  const MAX_X = TABLE_W - FELT_INSET - CARD_W
  const MAX_Y = TABLE_H - FELT_INSET - CARD_H
  const clampX = (v) => Math.min(Math.max(v, MIN_X), MAX_X)
  const clampY = (v) => Math.min(Math.max(v, MIN_Y), MAX_Y)

  // Empile la sélection en une pile nette au barycentre du groupe. MoveMany
  // préserve l'ordre Z relatif : la pile garde son ordre de superposition.
  function stackSelection() {
    const cards = table.filter((c) => selected.has(c.id))
    if (cards.length < 2) return
    const cx = clampX(Math.round(cards.reduce((s, c) => s + c.x, 0) / cards.length))
    const cy = clampY(Math.round(cards.reduce((s, c) => s + c.y, 0) / cards.length))
    dispatch('moveMany', { items: cards.map((c) => ({ cardId: c.id, x: cx, y: cy })) })
  }

  // Étale la sélection en une rangée horizontale centrée sur le barycentre,
  // ordonnée par Z croissant (la carte du dessus finit à droite). L'espacement
  // se resserre si la rangée déborderait de la table.
  function spreadSelection() {
    const cards = table.filter((c) => selected.has(c.id)).sort((a, b) => a.z - b.z)
    if (cards.length < 2) return
    // Largeur utile = l'amplitude des coins admissibles, pas celle du feutre :
    // la dernière carte de la rangée déborde encore de CARD_W au-delà de son
    // propre coin, déjà retranché dans MAX_X.
    const spacing = Math.min(30, (MAX_X - MIN_X) / (cards.length - 1))
    const cx = cards.reduce((s, c) => s + c.x, 0) / cards.length
    const cy = clampY(Math.round(cards.reduce((s, c) => s + c.y, 0) / cards.length))
    const x0 = cx - ((cards.length - 1) * spacing) / 2
    dispatch('moveMany', {
      items: cards.map((c, i) => ({ cardId: c.id, x: Math.round(clampX(x0 + i * spacing)), y: cy })),
    })
  }

  function flipSelection() {
    if (selected.size > 0) dispatch('flipMany', { cardIds: [...selected] })
  }

  // Renvoie la sélection SOUS le sabot : les cartes ne ressortiront qu'en
  // dernier. C'est l'issue offerte au joueur qui abandonne et laisse sa main
  // sur le tapis — les autres peuvent alors la ranger sans la regarder — mais
  // l'action vaut pour n'importe quelle sélection. À ne pas confondre avec un
  // glisser vers le sabot, qui dépose au sommet.
  function selectionToSabotBottom() {
    if (selected.size === 0) return
    dispatch('sabotBottomMany', { cardIds: [...selected] })
    selected = new Set()
  }
  function clearSelection() {
    selected = new Set()
  }

  // Raccourcis de sélection (hors champs de saisie, ex. chat) : Échap vide la
  // sélection, F retourne toutes les cartes sélectionnées d'un coup.
  function onGlobalKey(e) {
    const tag = e.target && e.target.tagName
    if (tag === 'INPUT' || tag === 'TEXTAREA') return
    if (selected.size === 0) return
    if (e.key === 'Escape') {
      selected = new Set()
    } else if (e.key === 'f' || e.key === 'F') {
      dispatch('flipMany', { cardIds: [...selected] })
    }
  }

  // ---- Rectangle de sélection (marquee) -------------------------------------
  // Le tracé ne démarre que sur le fond (table ou feutre) : les pointerdown
  // des cartes, avatars et sabot remontent aussi jusqu'ici par bouillonnement,
  // mais avec un target différent — il ne faut pas les confondre avec un début
  // de rectangle.
  function onTablePointerDown(e) {
    if (e.button !== 0 && e.pointerType === 'mouse') return
    const t = e.target
    if (t !== tableEl && !(t.classList && t.classList.contains('felt'))) return
    refreshRect()
    const x = e.clientX - tableRect.left
    const y = e.clientY - tableRect.top
    marquee = { x0: x, y0: y, x1: x, y1: y }
    marqueeBase = e.shiftKey ? new Set(selected) : new Set()
    tableEl.setPointerCapture(e.pointerId)
  }
  function onTablePointerMove(e) {
    if (!marquee) return
    marquee = { ...marquee, x1: e.clientX - tableRect.left, y1: e.clientY - tableRect.top }
    const xa = Math.min(marquee.x0, marquee.x1), xb = Math.max(marquee.x0, marquee.x1)
    const ya = Math.min(marquee.y0, marquee.y1), yb = Math.max(marquee.y0, marquee.y1)
    // Une carte est retenue si son point d'ancrage (c.x, c.y) tombe dans le
    // rectangle. Cet ancrage est le COIN supérieur gauche, pas le centre
    // (cf. `.card-anchor`) : il faut donc balayer le haut-gauche d'une carte
    // pour l'attraper, pas son milieu. Comportement inchangé, mais le
    // commentaire précédent affirmait le contraire.
    const next = new Set(marqueeBase)
    for (const c of table) {
      if (c.x >= xa && c.x <= xb && c.y >= ya && c.y <= yb) next.add(c.id)
    }
    selected = next
  }
  function onTablePointerUp(e) {
    if (!marquee) return
    try { tableEl.releasePointerCapture(e.pointerId) } catch {}
    // Simple clic dans le vide (aucun tracé réel), sans shift : désélection.
    const traced = Math.abs(marquee.x1 - marquee.x0) > 4 || Math.abs(marquee.y1 - marquee.y0) > 4
    if (!traced && !e.shiftKey) selected = new Set()
    marquee = null
  }

  // ---- Aimantage entre cartes proches (placement côte à côte) --------------
  // Deux cartes exactement côte à côte ont leurs points d'ancrage espacés d'une
  // largeur (horizontalement) ou d'une hauteur (verticalement) de carte. Le
  // calcul ci-dessous est purement relatif : il reste juste que l'ancrage
  // désigne un coin ou un centre.
  const SNAP_DIST = 22 // px : au-delà, pas d'aimantage (mouvement libre)

  // snapPosition ajuste (x,y) pour coller exactement au bord d'une carte
  // voisine si le point de relâchement en est suffisamment proche. `exclude`
  // (id seul ou Set d'ids) écarte de la recherche la carte déplacée — ou tout
  // un groupe en déplacement, qui ne doit pas s'aimanter sur lui-même.
  function snapPosition(x, y, exclude) {
    if (!snapEnabled) return { x, y }
    const excluded = exclude instanceof Set ? exclude : new Set(exclude ? [exclude] : [])
    let best = null
    let bestDist = SNAP_DIST
    for (const other of table) {
      if (excluded.has(other.id)) continue
      const candidates = [
        { x: other.x + CARD_W, y: other.y }, // à droite du voisin
        { x: other.x - CARD_W, y: other.y }, // à gauche du voisin
        { x: other.x, y: other.y + CARD_H }, // sous le voisin
        { x: other.x, y: other.y - CARD_H }, // au-dessus du voisin
      ]
      for (const cand of candidates) {
        const d = Math.hypot(cand.x - x, cand.y - y)
        if (d < bestDist) {
          bestDist = d
          best = cand
        }
      }
    }
    return best ?? { x, y }
  }

  // ---- Hit-test au drop : route vers la bonne action serveur ----
  // trackedPos : position d'ancrage de la carte réellement suivie pendant le
  // drag (cf. Card.svelte curX/curY), à utiliser pour le placement sur la
  // table. Le point du pointeur (clientX/clientY) ne sert qu'au hit-test de
  // la cible (table/sabot/avatar/main) : l'utiliser aussi comme coordonnées
  // finales recentrerait la carte sous le curseur, ignorant le décalage de
  // prise.
  function resolveDrop(clientX, clientY, cardId, trackedPos) {
    refreshRect()
    const hit = dropAt(clientX, clientY, { tableRect })
    if (!hit) {
      // Hors de toute cible : on annule (la carte reste à sa position serveur).
      return false
    }
    switch (hit.target) {
      case TARGETS.TABLE: {
        // Pose/replace sur le tapis à la position suivie pendant le drag,
        // aimantée contre une carte voisine si suffisamment proche.
        const pos = trackedPos ?? hit
        const snapped = snapPosition(pos.x, pos.y, cardId)
        dispatch('move', { cardId, x: snapped.x, y: snapped.y })
        return true
      }
      case TARGETS.SABOT:
        dispatch('transfer', { cardId, target: TARGETS.SABOT })
        return true
      case TARGETS.AVATAR:
        // Don de carte : elle passe dans la main privée du joueur cible.
        dispatch('transfer', { cardId, target: TARGETS.AVATAR, ownerId: hit.ownerId })
        return true
      case TARGETS.HAND:
        // Vers ma propre main (zone main basse).
        dispatch('transfer', { cardId, target: TARGETS.HAND, ownerId: myUserId })
        return true
    }
    return false
  }

  // ---- Drag groupé : positions live locales des cartes "suiveuses" ----------
  // La carte saisie (ancre) est déjà déplacée visuellement par Card.svelte
  // (transform GSAP) ; les autres cartes sélectionnées suivent via ce
  // dictionnaire, conservé après le drop jusqu'au prochain snapshot serveur
  // (sinon elles sauteraient à leur ancienne position en attendant la
  // confirmation, exactement le problème que Card.svelte évite pour l'ancre).
  let groupDrag = {}
  let groupDragging = false
  $: table, releaseGroupDrag()
  function releaseGroupDrag() {
    if (!groupDragging && Object.keys(groupDrag).length) groupDrag = {}
  }

  // groupItems calcule la position cible de chaque carte sélectionnée quand
  // l'ancre est en (ax, ay) : le groupe suit le même décalage, les positions
  // relatives sont préservées.
  function groupItems(anchorId, ax, ay) {
    const anchor = table.find((c) => c.id === anchorId)
    if (!anchor) return null
    const dx = ax - anchor.x, dy = ay - anchor.y
    return table
      .filter((c) => selected.has(c.id))
      .map((c) => ({ cardId: c.id, x: c.x + dx, y: c.y + dy }))
  }

  function setGroupDrag(items, anchorId) {
    const gd = {}
    for (const it of items) {
      if (it.cardId !== anchorId) gd[it.cardId] = { x: it.x, y: it.y }
    }
    groupDrag = gd
  }

  // Drop d'un groupe. Sur le tapis : déplacement groupé, avec aimantage
  // appliqué à la carte ancre uniquement (le groupe suit le même décalage,
  // sinon chaque carte s'aimanterait de son côté et la disposition relative
  // serait détruite). Sur le sabot, un avatar ou la main : transfert groupé —
  // toute la sélection change de zone (ramasser une levée, rendre une pile au
  // sabot, donner plusieurs cartes d'un geste).
  function resolveGroupDrop(clientX, clientY, anchorId, trackedPos) {
    refreshRect()
    groupDragging = false
    const hit = dropAt(clientX, clientY, { tableRect })
    if (!hit) {
      groupDrag = {}
      return false
    }
    switch (hit.target) {
      case TARGETS.TABLE: {
        const pos = trackedPos ?? hit
        const snapped = snapPosition(pos.x, pos.y, selected)
        const items = groupItems(anchorId, snapped.x, snapped.y)
        if (!items) {
          groupDrag = {}
          return false
        }
        setGroupDrag(items, anchorId)
        dispatch('moveMany', { items })
        return true
      }
      case TARGETS.SABOT:
        dispatch('transferMany', { cardIds: [...selected], target: TARGETS.SABOT })
        return true
      case TARGETS.AVATAR:
        dispatch('transferMany', { cardIds: [...selected], target: TARGETS.AVATAR, ownerId: hit.ownerId })
        return true
      case TARGETS.HAND:
        dispatch('transferMany', { cardIds: [...selected], target: TARGETS.HAND, ownerId: myUserId })
        return true
    }
    groupDrag = {}
    return false
  }

  // ---- Handlers des cartes de table ----
  function onCardDrag(e) {
    const { cardId, x, y } = e.detail
    // Carte membre d'une sélection multiple : tout le groupe suit (positions
    // locales + diffusion groupée aux autres clients).
    if (selected.has(cardId) && selected.size > 1) {
      const items = groupItems(cardId, x, y)
      if (items) {
        groupDragging = true
        setGroupDrag(items, cardId)
        dispatch('dragMany', { items })
        return
      }
    }
    // Position live locale + diffusion aux autres clients (fluidité).
    dispatch('drag', { cardId, x, y })
  }
  function onCardDrop(e) {
    const { cardId, clientX, clientY, x, y } = e.detail
    if (selected.has(cardId) && selected.size > 1) {
      e.detail.accepted = resolveGroupDrop(clientX, clientY, cardId, { x, y })
    } else {
      e.detail.accepted = resolveDrop(clientX, clientY, cardId, { x, y })
    }
    // Drop annulé (hors de toute cible) : les autres clients suivaient le
    // drag en live et garderaient sinon les cartes figées à la dernière
    // position survolée (aucune diffusion d'état ne viendra les corriger,
    // rien n'a changé côté serveur). On leur demande d'effacer le live.
    if (!e.detail.accepted) dispatch('dragEnd', {})
  }
  function onCardFlip(e) {
    // Double-clic sur une carte de la sélection : toute la sélection se
    // retourne d'un coup (geste "je retourne ma levée"). Sinon, flip simple.
    if (selected.has(e.detail.cardId) && selected.size > 1) {
      dispatch('flipMany', { cardIds: [...selected] })
    } else {
      dispatch('flip', e.detail)
    }
  }
  function onCardFront(e) { dispatch('front', e.detail) }
  function onCardRotate(e) { dispatch('rotate', e.detail) }

  // ---- Handlers du sabot ----
  function onSabotDraw(e) {
    const { clientX, clientY, ghostX, ghostY } = e.detail
    refreshRect()
    const hit = dropAt(clientX, clientY, { tableRect })
    if (!hit) return
    switch (hit.target) {
      case TARGETS.TABLE: {
        // Même parti pris que resolveDrop pour une carte du tapis : le pointeur
        // ne sert qu'au hit-test de la cible, jamais de position finale. Celle
        // du fantôme est la seule qui pose la carte là où le joueur la voit.
        // Son coin sup-gauche se convertit directement en coordonnées tapis :
        // c'est bien un coin qu'attend le rendu, cf. `.card-anchor` plus bas.
        const pos = (tableRect && ghostX != null)
          ? { x: ghostX - tableRect.left, y: ghostY - tableRect.top }
          : hit
        const snapped = snapPosition(pos.x, pos.y, null)
        dispatch('sabotDraw', { target: TARGETS.TABLE, x: snapped.x, y: snapped.y })
        break
      }
      case TARGETS.AVATAR:
        dispatch('sabotDraw', { target: TARGETS.AVATAR, ownerId: hit.ownerId })
        break
      case TARGETS.HAND:
        dispatch('sabotDraw', { target: TARGETS.HAND, ownerId: myUserId })
        break
      // drop sur sabot lui-même : no-op.
    }
  }

  // ---- Position rendue d'une carte de table ---------------------------------
  // Priorité : drag groupé local > drag live distant > position serveur.
  // Dictionnaire dérivé (plutôt qu'une fonction appelée dans le template) pour
  // que Svelte re-rende dès que l'une des trois sources change.
  $: posById = derivePositions(table, livePos, groupDrag)
  function derivePositions(tbl, live, group) {
    const out = {}
    for (const c of tbl) out[c.id] = group[c.id] ?? live[c.id] ?? { x: c.x, y: c.y }
    return out
  }
</script>

<div class="table-scroll">
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div
    bind:this={tableEl}
    class="table"
    data-drop="table"
    on:pointerdown={onTablePointerDown}
    on:pointermove={onTablePointerMove}
    on:pointerup={onTablePointerUp}
    on:pointercancel={onTablePointerUp}
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
      <div class="card-anchor" style="left:{posById[card.id].x}px; top:{posById[card.id].y}px; z-index:{card.z || 1};">
        <Card
          c={card}
          zone="table"
          selected={selected.has(card.id)}
          on:drag={onCardDrag}
          on:drop={onCardDrop}
          on:flip={onCardFlip}
          on:front={onCardFront}
          on:rotate={onCardRotate}
          on:toggleselect={onToggleSelect}
          on:selectpile={onSelectPile}
        />
      </div>
    {/each}

    <!-- Rectangle de sélection en cours de tracé -->
    {#if marquee}
      <div
        class="marquee"
        style="left:{Math.min(marquee.x0, marquee.x1)}px; top:{Math.min(marquee.y0, marquee.y1)}px; width:{Math.abs(marquee.x1 - marquee.x0)}px; height:{Math.abs(marquee.y1 - marquee.y0)}px;"
      ></div>
    {/if}

    <!-- Badge + actions de sélection multiple -->
    {#if selected.size > 0}
      <div class="sel-chip">
        <span>{selected.size} carte{selected.size > 1 ? 's' : ''}</span>
        {#if selected.size > 1}
          <button title="Regrouper la sélection en une pile" on:click={stackSelection}>Empiler</button>
          <button title="Disposer la sélection en rangée" on:click={spreadSelection}>Étaler</button>
        {/if}
        <button title="Retourner toutes les cartes sélectionnées (F)" on:click={flipSelection}>Retourner</button>
        <button
          title="Remettre la sélection sous le sabot : ces cartes ne ressortiront qu'en dernier"
          on:click={selectionToSabotBottom}
        >Au fond du sabot</button>
        <button title="Désélectionner (Échap)" on:click={clearSelection}>✕</button>
      </div>
    {/if}
  </div>
</div>

<style>
  .table-scroll {
    flex: 1;
    overflow: auto;
    position: relative;
    background: #1a120b;
  }
  .table {
    position: relative;
    width: 1160px;
    min-height: 800px;
    margin: 0 auto;
    /* pourtour "bois" : les avatars (assis autour, sur des chaises) reposent
       sur cette zone, le feutre vert n'occupant que le centre de la table.
       Largeur du pourtour ~ largeur du badge joueur (64px, cf. Avatar.svelte
       --size), pour que la zone bois serve juste à accueillir les avatars. */
    background: linear-gradient(160deg, #8a5a30 0%, #6b4423 55%, #4a2d14 100%);
    border-radius: 24px;
    box-shadow: inset 0 0 50px rgba(0,0,0,0.45), 0 12px 30px rgba(0,0,0,0.5);
  }
  .felt {
    position: absolute;
    inset: 80px;
    border-radius: 18px;
    background: radial-gradient(circle at 50% 45%, #1f7a52 0%, #135a3c 55%, #0a3a26 100%);
    box-shadow: inset 0 0 120px rgba(0,0,0,0.5), 0 4px 14px rgba(0,0,0,0.5);
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
  .marquee {
    position: absolute;
    border: 1.5px dashed rgba(255, 210, 122, 0.9);
    background: rgba(255, 210, 122, 0.12);
    border-radius: 4px;
    pointer-events: none;
    z-index: 5000;
  }
  .sel-chip {
    position: absolute;
    top: 14px;
    left: 50%;
    transform: translateX(-50%);
    display: flex;
    align-items: center;
    gap: 6px;
    background: rgba(0, 0, 0, 0.55);
    color: #ffd27a;
    padding: 4px 8px 4px 12px;
    border-radius: 999px;
    font-family: system-ui, sans-serif;
    font-size: .8rem;
    z-index: 5001;
    white-space: nowrap;
  }
  .sel-chip button {
    border: 1px solid rgba(255, 210, 122, 0.4);
    background: rgba(255, 210, 122, 0.12);
    color: #ffd27a;
    padding: 2px 9px;
    border-radius: 999px;
    font-size: .75rem;
    cursor: pointer;
  }
  .sel-chip button:hover { background: rgba(255, 210, 122, 0.28); }
  .card-anchor {
    position: absolute;
    /* ATTENTION — les cartes sont positionnées par leur COIN supérieur gauche,
       pas par leur centre. Un `transform: translate(-50%, -50%)` figurait ici
       avec un commentaire affirmant le contraire ; il etait sans effet, et le
       commentaire faux. L'unique enfant de cette ancre (`.card-slot`) est en
       position absolute, donc hors flux : l'ancre mesure 0x0, et un pourcentage
       calcule sur 0 vaut 0. Le translate a ete retire plutot que corrigé, car
       le reste du code (aimantage, detection de pile, drag) est écrit et réglé
       pour des coins ; le faire fonctionner deplacerait chaque carte d'une
       demi-carte. Toute position envoyée au serveur doit donc être un coin. */
  }
</style>
