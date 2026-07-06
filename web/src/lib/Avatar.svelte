<script>
  // Avatar 64×64 (§8) : représente un joueur à table. Cible de drop : déposer
  // une carte sur un avatar la transfère dans la main privée du joueur (§6).
  import { createEventDispatcher } from 'svelte'

  export let player = { userId: '', name: '', ax: 0, ay: 0, handCount: 0 }
  export let isMe = false
  export let size = 64

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
  data-drop="avatar"
  data-owner={player.userId}
  style="--size:{size}px; --hue:{hue}; left:{player.ax}px; top:{player.ay}px;"
  title="{player.name} (déposer une carte ici la lui donne)"
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
  <div class="name">{player.name}{isMe ? ' (vous)' : ''}</div>
</div>

<style>
  .avatar {
    position: absolute;
    width: var(--size);
    transform: translate(-50%, -50%);
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    cursor: default;
    user-select: none;
  }
  .medal {
    position: relative;
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
  .handcount {
    position: absolute;
    top: -6px;
    right: -6px;
    background: #2f9e63;
    color: #fff;
    font-size: 11px;
    font-weight: 700;
    min-width: 20px;
    height: 20px;
    padding: 0 5px;
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
  .name {
    background: rgba(0,0,0,0.55);
    color: #fff;
    font-size: 11px;
    padding: 1px 8px;
    border-radius: 999px;
    font-family: system-ui, sans-serif;
    white-space: nowrap;
    max-width: 120px;
    overflow: hidden;
    text-overflow: ellipsis;
  }
</style>
