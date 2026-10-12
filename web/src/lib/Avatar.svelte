<script>
  // Avatar 64×64 (§8) : représente un joueur à table. Cible de drop : déposer
  // une carte sur un avatar la transfère dans la main privée du joueur (§6).
  import { createEventDispatcher } from 'svelte'
  import { TABLE_W, FELT_INSET, projectOnSeatRing } from './geometry.js'

  export let player = { userId: '', name: '', ax: 0, ay: 0, handCount: 0 }
  export let isMe = false
  // Le joueur peut faire glisser SON avatar le long du bord de la table pour
  // changer de place (Table.svelte ne l'active que pour soi, à table).
  export let movable = false
  // Diamètre extérieur du cercle, liseré compris (box-sizing: border-box).
  // 70 = l'ancien cercle de 64px plus son liseré de 3px de chaque côté : même
  // aspect, mais la taille dit désormais vraiment ce qu'occupe le cercle, et
  // l'ancrage sur son centre tombe juste (le liseré décalait tout de 3px).
  export let size = 70

  const dispatch = createEventDispatcher()

  // Détermine une couleur d'avatar stable par nom (teinte déterministe).
  $: hue = hashHue(player.name || player.userId || '?')
  function hashHue(s) {
    let h = 0
    for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) % 360
    return h
  }

  // Initiales affichées dans le médaillon.
  $: initials = (player.name || '?')
    .split(/\s+/)
    .map((w) => w[0])
    .join('')
    .slice(0, 2)
    .toUpperCase()

  // Côté de l'étiquette du nom. Centrée sous l'avatar en temps normal ; mais
  // les sièges latéraux sont à 44px du bord de la table, et un nom de 24px en
  // déborderait d'autant plus qu'il est long — ce débordement ferait défiler le
  // tapis. Près d'un bord, l'étiquette part donc du bord de l'avatar vers
  // l'intérieur du tapis.
  const EDGE = 140
  $: side = pos.x < EDGE ? 'left' : pos.x > TABLE_W - EDGE ? 'right' : ''
  // Siège du grand côté nord (centre du cercle sur le bord haut du feutre) :
  // le nom passe au-dessus du cercle, hors de la table, au lieu de couvrir le
  // feutre.
  $: north = pos.y < FELT_INSET + 1

  // ---- Changement de place (glisser son avatar) ------------------------------
  // Pendant le geste, l'avatar suit le doigt, ramené sur le pourtour des sièges
  // (projectOnSeatRing) ; au lâcher, la place part au serveur, qui a le dernier
  // mot et la diffuse. La position du geste est gardée jusqu'à ce retour, pour
  // que l'avatar ne revienne pas un instant à son ancienne place.
  let rootEl
  let dragPos = null      // position suivie pendant le geste, puis en attente
  let dragging = false
  let moved = false
  let startX = 0, startY = 0
  let pendingTimer = null
  $: pos = dragPos ?? { x: player.ax, y: player.ay }
  // Le serveur a répondu (nouvelle place) : on quitte la position du geste.
  // Seul un vrai changement de place compte : l'état est rediffusé à chaque
  // geste de n'importe quel joueur, et le moindre envoi ramenait sinon
  // l'avatar un instant à son ancienne place.
  let lastAx = player.ax, lastAy = player.ay
  $: settle(player.ax, player.ay)
  function settle(ax, ay) {
    if (ax === lastAx && ay === lastAy) return
    lastAx = ax; lastAy = ay
    if (!dragging && dragPos) { dragPos = null; clearTimeout(pendingTimer) }
  }

  function tablePoint(e) {
    const table = rootEl?.closest('[data-scale]')
    if (!table) return null
    const r = table.getBoundingClientRect()
    const s = Number(table.dataset.scale) || 1
    return projectOnSeatRing((e.clientX - r.left) / s, (e.clientY - r.top) / s)
  }
  function onPointerDown(e) {
    if (!movable) return
    if (e.button !== 0 && e.pointerType === 'mouse') return
    dragging = true
    moved = false
    startX = e.clientX
    startY = e.clientY
    e.currentTarget.setPointerCapture(e.pointerId)
    e.preventDefault()
    e.stopPropagation()
  }
  function onPointerMove(e) {
    if (!dragging) return
    if (!moved && Math.hypot(e.clientX - startX, e.clientY - startY) < 5) return
    moved = true
    dragPos = tablePoint(e)
  }
  function onPointerUp(e) {
    if (!dragging) return
    dragging = false
    try { e.currentTarget.releasePointerCapture(e.pointerId) } catch {}
    if (!moved || !dragPos) { dragPos = null; return }
    dispatch('seat', { x: dragPos.x, y: dragPos.y })
    // Filet : si le serveur ne renvoie rien (place identique, refus), on
    // revient à sa position plutôt que de rester figé sur celle du geste.
    clearTimeout(pendingTimer)
    pendingTimer = setTimeout(() => { dragPos = null }, 1500)
  }

  // Feedback visuel au survol d'un drag.
  let hovered = false
  function onDragEnter(e) { e.preventDefault(); hovered = true }
  function onDragLeave() { hovered = false }
  function onDragOver(e) { e.preventDefault() }
  function onDrop(e) {
    e.preventDefault()
    hovered = false
    // Le drop réel est géré par le hit-test global (data-drop/data-owner).
  }
</script>

<div
  class="avatar"
  class:is-me={isMe}
  class:hovered
  class:abandoned={player.abandoned}
  class:edge-left={side === 'left'}
  class:edge-right={side === 'right'}
  class:north
  class:movable
  class:moving={dragging && moved}
  bind:this={rootEl}
  {...(player.abandoned ? {} : { 'data-drop': 'avatar' })}
  data-owner={player.userId}
  style="--size:{size}px; --hue:{hue}; left:{pos.x}px; top:{pos.y}px;"
  title={player.abandoned
    ? `${player.name} a quitté la partie`
    : movable
      ? `${player.name} — glissez votre avatar le long de la table pour changer de place`
      : `${player.name} (déposer une carte ici la lui donne)`}
  on:pointerdown={onPointerDown}
  on:pointermove={onPointerMove}
  on:pointerup={onPointerUp}
  on:pointercancel={onPointerUp}
  on:dragenter={onDragEnter}
  on:dragleave={onDragLeave}
  on:dragover={onDragOver}
  on:drop={onDrop}
>
  <div class="medal">
    {initials}
    <!-- Nombre de cartes en main (comme le badge du sabot) : le compte est
         public, jamais les cartes elles-mêmes (§ vie privée de la main). -->
    <div class="handcount" title="{player.handCount ?? 0} carte{(player.handCount ?? 0) > 1 ? 's' : ''} en main">{player.handCount ?? 0}</div>
  </div>
  <div class="name">{player.name}{player.abandoned ? ' — parti' : isMe ? ' (vous)' : ''}</div>
</div>

<style>
  /* (left, top) = centre du CERCLE, et non du bloc cercle + nom : le serveur
     pose ce centre sur le bord du feutre (layoutAvatar), pour que le cercle y
     empiète de moitié. Au nord, le nom passe au-dessus (column-reverse) : le
     décalage vertical part alors du bas du bloc. */
  .avatar {
    position: absolute;
    width: var(--size);
    transform: translate(-50%, calc(var(--size) / -2));
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    cursor: default;
    user-select: none;
  }
  /* Sièges près d'un bord : l'étiquette déborde vers le tapis, pas hors de la
     table (cf. `side` dans le script). */
  .avatar.edge-left { align-items: flex-start; }
  .avatar.movable { cursor: grab; touch-action: none; }
  .avatar.moving { cursor: grabbing; z-index: 6000; }
  .avatar.moving .medal { box-shadow: 0 0 0 6px rgba(255, 210, 122, 0.35), 0 8px 18px rgba(0,0,0,0.55); }
  .avatar.north { flex-direction: column-reverse; transform: translate(-50%, calc(-100% + var(--size) / 2)); }
  .avatar.edge-right { align-items: flex-end; }
  .medal {
    position: relative;
    box-sizing: border-box;
    width: var(--size);
    height: var(--size);
    border-radius: 50%;
    background: hsl(var(--hue) 50% 45%);
    border: 3px solid rgba(255,255,255,0.85);
    box-shadow: 0 4px 10px rgba(0,0,0,0.4);
    display: grid;
    place-items: center;
    color: #fff;
    font-weight: 700;
    font-size: calc(var(--size) * 0.36);
    font-family: system-ui, sans-serif;
    transition: transform .12s ease, box-shadow .12s ease;
  }
  /* Même taille que le compteur du sabot (.badge dans Sabot.svelte) : les deux
     disent la même chose, un nombre de cartes. Toute modification ici doit y
     être reportée. */
  .handcount {
    position: absolute;
    top: -6px;
    right: -6px;
    background: #2f9e63;
    color: #fff;
    font-size: 15px;
    font-weight: 700;
    min-width: 26px;
    height: 26px;
    padding: 0 6px;
    border-radius: 999px;
    display: grid;
    place-items: center;
    font-family: system-ui, sans-serif;
    box-shadow: 0 2px 6px rgba(0,0,0,0.4);
  }
  .avatar.is-me .medal { border-color: #ffd27a; }
  .avatar.hovered .medal {
    transform: scale(1.1);
    box-shadow: 0 0 0 6px rgba(255, 210, 122, 0.35), 0 6px 14px rgba(0,0,0,0.5);
  }

  /* Chaise d'un joueur qui a quitté la partie : la place reste marquée, mais
     désaturée et estompée — on voit qu'elle est vide pour de bon, sans qu'elle
     attire l'œil. */
  .avatar.abandoned { opacity: .45; }
  .avatar.abandoned .medal {
    filter: grayscale(1);
    border-style: dashed;
    border-color: rgba(255,255,255,0.5);
    box-shadow: none;
  }
  .avatar.abandoned .handcount { display: none; }
  /* Un partant n'est plus une cible de don : le survol ne doit rien promettre.
     L'attribut data-drop lui est retiré plus haut, donc dropAt() ne le voit
     même plus — et le serveur refuse le transfert de son côté. */
  .avatar.abandoned.hovered .medal { transform: none; box-shadow: none; }
  /* Tailles en pixels du TAPIS, réduit ensuite avec lui (Table.svelte) : sur
     tablette l'échelle tourne autour de 0,6, et les 11px d'origine tombaient
     sous les 7px à l'écran. 24px y donnent environ 15px. */
  .name {
    background: rgba(0,0,0,0.6);
    color: #fff;
    font-size: 24px;
    font-weight: 600;
    padding: 2px 12px;
    border-radius: 999px;
    font-family: system-ui, sans-serif;
    white-space: nowrap;
    max-width: 260px;
    overflow: hidden;
    text-overflow: ellipsis;
  }
</style>
