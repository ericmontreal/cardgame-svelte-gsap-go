<script>
  // Zone chat (§9) : communication temps réel entre joueurs connectés. Messages
  // diffusés à tous les participants de la session. Pas de persistance.
  import { chatLog, loadChatHidden, saveChatHidden } from './store.js'
  import { afterUpdate, onMount } from 'svelte'
  import { gsap } from 'gsap'

  export let onSend = null  // (text) => void  : callback d'envoi vers le serveur

  let text = ''
  let listEl

  // ---- Repli du chat -------------------------------------------------------
  // Le chat occupe une colonne fixe de 260px prise sur le tapis. Le replier la
  // rend au jeu — la grille de App.svelte (`1fr auto`) suit la largeur du dock.
  // La préférence est conservée d'une session à l'autre.
  const OPEN_W = 260   // largeur du panneau déployé
  const RAIL_W = 30    // largeur de la languette une fois replié
  const SLIDE_S = 0.32 // durée du glissement

  let hidden = loadChatHidden()
  let dockEl, panelEl, railEl
  let anim = null

  // Messages arrivés pendant que le chat est replié. Sans ce compteur, replier
  // le chat revient à manquer les messages en silence : rien à l'écran ne
  // signalerait qu'on parle.
  let unread = 0
  let seenCount = $chatLog.length

  $: if (!hidden) {
    // Chat visible : tout est lu au fil de l'eau.
    seenCount = $chatLog.length
    unread = 0
  } else {
    unread = Math.max(0, $chatLog.length - seenCount)
  }

  // Un joueur ayant demandé moins d'animations est servi sans glissement : le
  // repli reste instantané, la fonction demeure entière.
  function prefersReducedMotion() {
    return typeof window !== 'undefined'
      && window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
  }

  // applyState place le dock dans l'état voulu. `animate` distingue le premier
  // rendu (pose immédiate, sinon le chat ferait une entrée gratuite à chaque
  // chargement) du basculement demandé par le joueur.
  //
  // Le panneau garde sa largeur pleine et sort en glissant, plutôt que d'être
  // écrasé : c'est le dock, en `overflow: hidden`, qui rétrécit et le rogne.
  // Sans cela le formulaire et les messages se comprimeraient pendant la
  // transition, ce qui se voit beaucoup.
  function applyState(animate) {
    if (!dockEl || !panelEl || !railEl) return
    anim?.kill()

    const to = hidden
      ? { width: RAIL_W, panelX: OPEN_W, panelAlpha: 0, railAlpha: 1 }
      : { width: OPEN_W, panelX: 0, panelAlpha: 1, railAlpha: 0 }

    if (!animate || prefersReducedMotion()) {
      gsap.set(dockEl, { width: to.width })
      // autoAlpha = opacité + visibility : un panneau replié sort aussi du
      // parcours de tabulation, sinon on tomberait sur un champ invisible.
      gsap.set(panelEl, { x: to.panelX, autoAlpha: to.panelAlpha })
      gsap.set(railEl, { autoAlpha: to.railAlpha })
      return
    }

    anim = gsap.timeline({ defaults: { duration: SLIDE_S, ease: 'power2.inOut' } })
    if (hidden) {
      anim
        .to(panelEl, { x: to.panelX, autoAlpha: to.panelAlpha }, 0)
        .to(dockEl, { width: to.width }, 0)
        .to(railEl, { autoAlpha: to.railAlpha, duration: SLIDE_S * 0.6 }, SLIDE_S * 0.4)
    } else {
      anim
        .to(railEl, { autoAlpha: to.railAlpha, duration: SLIDE_S * 0.4 }, 0)
        .to(dockEl, { width: to.width }, 0)
        .to(panelEl, { x: to.panelX, autoAlpha: to.panelAlpha }, 0)
    }
  }

  onMount(() => applyState(false))

  function toggle() {
    hidden = !hidden
    saveChatHidden(hidden)
    applyState(true)
  }

  // Auto-scroll vers le bas à chaque nouveau message. Le panneau reste monté
  // même replié (c'est ce qui permet de l'animer), donc `listEl` existe
  // toujours : faire défiler une liste masquée ne coûte rien et évite un saut
  // à la réouverture.
  afterUpdate(() => {
    if (listEl) listEl.scrollTop = listEl.scrollHeight
  })

  function submit(e) {
    e.preventDefault()
    const t = text.trim()
    if (!t || !onSend) return
    onSend(t)
    text = ''
  }

  // Formatage de l'horloge (HH:MM).
  function clock(at) {
    try {
      const d = new Date(at)
      return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    } catch {
      return ''
    }
  }
</script>

<!-- Le dock donne la largeur réelle de la colonne, et c'est lui qu'anime GSAP.
     Les deux vues restent montées en permanence : un {#if} les échangerait d'un
     coup et il n'y aurait rien à faire glisser. -->
<aside class="dock" bind:this={dockEl}>
  <!-- Chat replié : ne subsiste qu'une languette verticale, seule chose qui
       permette de le rouvrir. Elle porte le compte des messages non lus, sinon
       replier le chat reviendrait à manquer la conversation sans le savoir. -->
  <button class="reopen" bind:this={railEl} on:click={toggle} title="Afficher le chat">
    <span class="chev">‹</span>
    <span class="vert">Chat</span>
    {#if unread > 0}
      <span class="badge" title="{unread} message{unread > 1 ? 's' : ''} non lu{unread > 1 ? 's' : ''}">
        {unread > 99 ? '99+' : unread}
      </span>
    {/if}
  </button>

  <div class="panel" bind:this={panelEl}>
  <div class="head">
    <span>Chat</span>
    <button class="hide" on:click={toggle} title="Masquer le chat">›</button>
  </div>
  <div class="list" bind:this={listEl}>
    {#each $chatLog as msg (msg.at + '|' + msg.author + '|' + msg.text)}
      <div class="msg">
        <span class="meta"><b>{msg.author}</b> <time>{clock(msg.at)}</time></span>
        <span class="body">{msg.text}</span>
      </div>
    {:else}
      <div class="empty">Soyez le premier à parler…</div>
    {/each}
  </div>
  <form class="composer" on:submit={submit}>
    <input
      type="text"
      bind:value={text}
      placeholder="Votre message…"
      autocomplete="off"
      maxlength="500"
    />
    <button type="submit" disabled={!text.trim()}>Envoyer</button>
  </form>
  </div>
</aside>

<style>
  /* Le dock porte la largeur animée et rogne ce qui déborde : c'est ce
     rognage qui donne l'impression que le panneau sort en glissant, au lieu
     d'être comprimé sur place. Sa largeur est posée par GSAP au montage. */
  .dock {
    position: relative;
    height: 100%;
    overflow: hidden;
    background: rgba(8, 24, 18, 0.85);
    border-left: 2px solid rgba(255,255,255,0.1);
    font-family: system-ui, sans-serif;
    flex: none;
  }
  /* Le panneau garde sa largeur pleine quoi qu'il arrive : il est en absolu
     pour que le rétrécissement du dock ne le mette jamais en page à 30px. */
  .panel {
    position: absolute;
    inset: 0 auto 0 0;
    width: 260px;
    height: 100%;
    display: flex;
    flex-direction: column;
  }
  .head {
    padding: 10px 12px;
    color: #cfe;
    font-weight: 700;
    border-bottom: 1px solid rgba(255,255,255,0.1);
    background: rgba(0,0,0,0.3);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }
  .hide {
    border: 0; background: transparent; cursor: pointer;
    color: #cfe; opacity: .7;
    font-size: 1.2rem; line-height: 1;
    padding: 2px 6px; border-radius: 6px;
  }
  .hide:hover { opacity: 1; background: rgba(255,255,255,0.1); }

  /* ---- Languette du chat replié ---- */
  /* Occupe la largeur repliée, sous le panneau. GSAP la fait apparaître en fin
     de fermeture et disparaître au début de l'ouverture. */
  .reopen {
    position: absolute;
    inset: 0 auto 0 0;
    width: 30px;
    border: 0; background: transparent; cursor: pointer;
    color: #cfe; opacity: .75;
    display: flex; flex-direction: column; align-items: center; gap: 8px;
    padding: 10px 0 0;
  }
  .reopen:hover { opacity: 1; background: rgba(255,255,255,0.07); }
  .chev { font-size: 1.2rem; line-height: 1; }
  /* Libellé pivoté : le seul moyen de nommer la languette sans l'élargir. */
  .vert {
    writing-mode: vertical-rl;
    font-size: .75rem;
    font-weight: 700;
    letter-spacing: .08em;
  }
  .badge {
    background: #2f9e63;
    color: #fff;
    font-size: 10px;
    font-weight: 700;
    min-width: 18px;
    padding: 1px 4px;
    border-radius: 999px;
    box-shadow: 0 2px 6px rgba(0,0,0,0.4);
  }
  .list { flex: 1; overflow-y: auto; padding: 8px 10px; display: flex; flex-direction: column; gap: 8px; }
  .msg { display: flex; flex-direction: column; gap: 1px; }
  .meta { font-size: .72rem; opacity: .85; display: flex; gap: 6px; align-items: baseline; }
  .meta time { opacity: .7; font-weight: 400; }
  .body { color: #eef; font-size: .9rem; word-break: break-word; }
  .empty { color: rgba(255,255,255,0.4); font-size: .85rem; align-self: center; margin-top: 1rem; }
  .composer { display: flex; gap: 6px; padding: 8px; border-top: 1px solid rgba(255,255,255,0.1); }
  .composer input {
    flex: 1; padding: .5rem .6rem; border-radius: 7px;
    border: 1px solid rgba(255,255,255,0.15); background: rgba(0,0,0,0.3); color: #eef;
  }
  .composer input:focus { outline: 2px solid #4caa7a; border-color: transparent; }
  .composer button {
    border: 0; border-radius: 7px; padding: .5rem .8rem; cursor: pointer;
    background: #2f9e63; color: #fff; font-weight: 600;
  }
  .composer button:disabled { opacity: .5; cursor: not-allowed; }
</style>
