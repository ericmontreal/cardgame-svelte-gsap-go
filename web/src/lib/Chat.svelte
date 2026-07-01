<script>
  // Zone chat (§9) : communication temps réel entre joueurs connectés. Messages
  // diffusés à tous les participants de la session. Pas de persistance.
  import { chatLog, applyChat } from './store.js'
  import { createEventDispatcher, afterUpdate, onMount } from 'svelte'

  export let onSend = null  // (text) => void  : callback d'envoi vers le serveur

  let text = ''
  let listEl

  // Auto-scroll vers le bas à chaque nouveau message.
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

<aside class="chat">
  <div class="head">Chat</div>
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
</aside>

<style>
  .chat {
    width: 260px;
    min-width: 220px;
    height: 100%;
    display: flex;
    flex-direction: column;
    background: rgba(8, 24, 18, 0.85);
    border-left: 2px solid rgba(255,255,255,0.1);
    font-family: system-ui, sans-serif;
  }
  .head {
    padding: 10px 12px;
    color: #cfe;
    font-weight: 700;
    border-bottom: 1px solid rgba(255,255,255,0.1);
    background: rgba(0,0,0,0.3);
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
