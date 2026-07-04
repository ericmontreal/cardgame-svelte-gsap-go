<script>
  // Formulaire d'authentification (§10) : utilisateurs préenregistrés.
  // Aucune création de compte dynamique.
  import { createEventDispatcher } from 'svelte'
  import { login } from './store.js'

  export let seedHint = ''  // texte d'aide affiché sous le form (ex. comptes démo)

  const dispatch = createEventDispatcher()
  let username = ''
  let password = ''
  let error = ''
  let busy = false

  async function submit(e) {
    e.preventDefault()
    error = ''
    if (!username.trim() || !password) {
      error = 'Renseignez un identifiant et un mot de passe.'
      return
    }
    busy = true
    try {
      const session = await login(username.trim(), password)
      dispatch('success', session)
    } catch (err) {
      error = err.message || 'Échec de connexion'
    } finally {
      busy = false
    }
  }
</script>

<div class="login-wrap">
  <form class="login-card" on:submit={submit}>
    <h1>Table de cartes</h1>
    <p class="subtitle">Connectez-vous pour rejoindre la table.</p>

    <label>
      Identifiant
      <input
        type="text"
        bind:value={username}
        autocomplete="username"
        spellcheck="false"
        placeholder="alice"
        disabled={busy}
      />
    </label>

    <label>
      Mot de passe
      <input
        type="password"
        bind:value={password}
        autocomplete="current-password"
        placeholder="••••••"
        disabled={busy}
      />
    </label>

    {#if error}
      <p class="error">{error}</p>
    {/if}

    <button type="submit" disabled={busy}>
      {busy ? 'Connexion…' : 'Se connecter'}
    </button>

    {#if seedHint}
      <p class="hint">{seedHint}</p>
    {/if}
  </form>
</div>

<style>
  .login-wrap {
    min-height: 100vh;
    display: grid;
    place-items: center;
    background: radial-gradient(circle at 50% 30%, #1c6e4b 0%, #0d3a26 70%, #062016 100%);
    font-family: system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif;
  }
  .login-card {
    width: min(360px, 90vw);
    background: rgba(10, 30, 22, 0.9);
    color: #eef;
    padding: 2rem 1.75rem 1.5rem;
    border-radius: 14px;
    border: 1px solid rgba(255,255,255,0.08);
    box-shadow: 0 20px 60px rgba(0,0,0,0.45);
    display: flex;
    flex-direction: column;
    gap: 0.85rem;
  }
  h1 { margin: 0 0 .25rem; font-size: 1.5rem; }
  .subtitle { margin: 0 0 .5rem; opacity: .7; font-size: .9rem; }
  label { display: flex; flex-direction: column; gap: .3rem; font-size: .85rem; opacity: .9; }
  input {
    padding: .6rem .7rem; border-radius: 8px; border: 1px solid rgba(255,255,255,0.15);
    background: rgba(0,0,0,0.3); color: #eef; font-size: 1rem;
  }
  input:focus { outline: 2px solid #4caa7a; border-color: transparent; }
  button {
    margin-top: .4rem; padding: .7rem; border: 0; border-radius: 8px; cursor: pointer;
    background: #2f9e63; color: #fff; font-weight: 600; font-size: 1rem;
  }
  button:hover:not(:disabled) { background: #36b46f; }
  button:disabled { opacity: .6; cursor: progress; }
  .error { color: #ff9a9a; margin: 0; font-size: .85rem; }
  .hint { color: #9fbfb0; margin: .5rem 0 0; font-size: .78rem; line-height: 1.3; }
</style>
