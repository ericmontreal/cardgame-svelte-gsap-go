<script>
  // Menu d'initialisation (§4) : sélection par cases à cocher du nombre et du
  // type de jeux de cartes (valeurs, couleurs, jokers...). Inspiré de
  // web/config_jeu.html (cases à cocher par valeur de carte plutôt qu'une
  // plage de-à). Affiché en début de session et à chaque nouvelle partie ;
  // la configuration choisie est conservée (localStorage) tant que le bouton
  // "Réinitialiser" n'est pas cliqué.
  import { createEventDispatcher } from 'svelte'
  import { loadDeckConfig, saveDeckConfig, clearDeckConfig } from './store.js'

  export let seedHint = ''
  const dispatch = createEventDispatcher()

  // ---- Valeurs par défaut (jeu classique 52 cartes) ----
  const defaultSuits = { club: true, diamond: true, heart: true, spade: true }
  const defaultRanks = {
    1: true, 2: true, 3: true, 4: true, 5: true, 6: true, 7: true,
    8: true, 9: true, 10: true, 11: true, 12: true, 13: true,
  }
  const defaultDeckCount = 1
  const defaultJokers = 'none'
  const defaultShuffle = true

  // ---- État du formulaire, pré-rempli depuis la config persistée ----
  const persisted = loadDeckConfig()
  let suits = { ...defaultSuits, ...(persisted?.suits ?? {}) }
  let ranksChecked = { ...defaultRanks, ...(persisted?.ranks ?? {}) }
  let deckCount = persisted?.deckCount ?? defaultDeckCount
  let jokers = persisted?.jokers ?? defaultJokers
  let shuffle = persisted?.shuffle ?? defaultShuffle

  // Ordre d'affichage des valeurs, comme config_jeu.html : 2..10, Valets,
  // Dames, Rois, As (l'As en dernier, convention "carte forte").
  const rankOrder = [2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 1]
  const rankLabel = {
    1: 'As', 2: '2', 3: '3', 4: '4', 5: '5', 6: '6', 7: '7',
    8: '8', 9: '9', 10: '10', 11: 'Valets', 12: 'Dames', 13: 'Rois',
  }
  const suitGlyph = { club: '♣', diamond: '♦', heart: '♥', spade: '♠' }
  const suitName = { club: 'Trèfle', diamond: 'Carreau', heart: 'Cœur', spade: 'Pique' }

  $: selectedSuits = Object.keys(suits).filter((k) => suits[k])
  $: selectedRanks = rankOrder.filter((r) => ranksChecked[r])
  $: validSuits = selectedSuits.length > 0
  $: validRanks = selectedRanks.length > 0
  // Estimation du nombre de cartes (aperçu en direct).
  $: estCards =
    (validSuits && validRanks)
      ? deckCount * selectedSuits.length * selectedRanks.length + jokerCount()
      : 0

  function jokerCount() {
    if (jokers === 'both') return deckCount * 2
    if (jokers === 'black' || jokers === 'red') return deckCount
    return 0
  }

  function start() {
    if (!validSuits || !validRanks) return
    saveDeckConfig({ suits: { ...suits }, ranks: { ...ranksChecked }, deckCount, jokers, shuffle })
    dispatch('start', {
      config: {
        deckCount,
        suits: selectedSuits,
        ranks: selectedRanks,
        jokers,
      },
      shuffle,
    })
  }

  function reset() {
    clearDeckConfig()
    suits = { ...defaultSuits }
    ranksChecked = { ...defaultRanks }
    deckCount = defaultDeckCount
    jokers = defaultJokers
    shuffle = defaultShuffle
  }
</script>

<div class="init-wrap">
  <div class="init-card">
    <h2>Préparation du sabot</h2>
    <p class="subtitle">Choisissez la composition du jeu, puis ouvrez la table.</p>

    <fieldset>
      <legend>Valeurs des cartes</legend>
      <div class="row ranks">
        {#each rankOrder as r}
          <label class="chip" class:checked={ranksChecked[r]}>
            <input type="checkbox" bind:checked={ranksChecked[r]} />
            <span>{rankLabel[r]}</span>
          </label>
        {/each}
      </div>
    </fieldset>

    <fieldset>
      <legend>Couleurs</legend>
      <div class="row suits">
        {#each Object.keys(suits) as s}
          <label class="chip" class:checked={suits[s]}>
            <input type="checkbox" bind:checked={suits[s]} />
            <span class="glyph {s}">{suitGlyph[s]}</span>
            <span class="nm">{suitName[s]}</span>
          </label>
        {/each}
      </div>
    </fieldset>

    <fieldset>
      <legend>Nombre de jeux</legend>
      <div class="row deck">
        <button type="button" on:click={() => (deckCount = Math.max(1, deckCount - 1))}>−</button>
        <strong class="count">{deckCount}</strong>
        <button type="button" on:click={() => (deckCount = Math.min(8, deckCount + 1))}>+</button>
      </div>
    </fieldset>

    <fieldset>
      <legend>Jokers</legend>
      <div class="row jokers">
        {#each [['none', 'Aucun'], ['black', 'Noir'], ['red', 'Rouge'], ['both', 'Noir + Rouge']] as [val, lbl]}
          <label class="chip" class:checked={jokers === val}>
            <input type="radio" name="jokers" value={val} bind:group={jokers} />
            <span>{lbl}</span>
          </label>
        {/each}
      </div>
    </fieldset>

    <fieldset>
      <legend>Options</legend>
      <label class="opt">
        <input type="checkbox" bind:checked={shuffle} />
        Mélanger le sabot à la création
      </label>
    </fieldset>

    <div class="summary">
      <span>Aperçu : <strong>{estCards}</strong> carte{estCards > 1 ? 's' : ''}</span>
      {#if !validSuits}<span class="warn">— sélectionnez au moins une couleur</span>
      {:else if !validRanks}<span class="warn">— sélectionnez au moins une valeur</span>{/if}
    </div>

    <div class="actions">
      <button class="reset" type="button" on:click={reset}>Réinitialiser</button>
      <button class="start" on:click={start} disabled={!validSuits || !validRanks}>
        Ouvrir la table
      </button>
    </div>
    {#if seedHint}<p class="hint">{seedHint}</p>{/if}
  </div>
</div>

<style>
  .init-wrap {
    min-height: 100vh; display: grid; place-items: center;
    background: radial-gradient(circle at 50% 30%, #1c6e4b 0%, #0d3a26 70%, #062016 100%);
    font-family: system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif;
  }
  .init-card {
    width: min(560px, 94vw);
    background: rgba(10, 30, 22, 0.92);
    color: #eef;
    padding: 1.6rem 1.6rem 1.3rem;
    border-radius: 14px;
    border: 1px solid rgba(255,255,255,0.08);
    box-shadow: 0 20px 60px rgba(0,0,0,0.45);
  }
  h2 { margin: 0 0 .2rem; font-size: 1.35rem; }
  .subtitle { margin: 0 0 1rem; opacity: .7; font-size: .88rem; }
  fieldset { border: 1px solid rgba(255,255,255,0.1); border-radius: 10px; padding: .7rem .9rem; margin: .6rem 0; }
  legend { padding: 0 .4rem; font-size: .8rem; opacity: .8; text-transform: uppercase; letter-spacing: .04em; }
  .row { display: flex; gap: .6rem; align-items: center; flex-wrap: wrap; }
  .row.ranks .chip { padding: .4rem .6rem; }
  .chip {
    display: inline-flex; align-items: center; gap: .4rem;
    padding: .45rem .7rem; border-radius: 999px; cursor: pointer;
    background: rgba(255,255,255,0.05); border: 1px solid rgba(255,255,255,0.1);
    font-size: .9rem; user-select: none;
  }
  .chip.checked { background: rgba(47, 158, 99, 0.35); border-color: #2f9e63; }
  .chip input { accent-color: #2f9e63; }
  .glyph { font-size: 1.15rem; }
  .glyph.heart, .glyph.diamond { color: #e8556a; }
  .glyph.club, .glyph.spade { color: #eef; }
  .nm { font-size: .85rem; }
  .deck button {
    width: 36px; height: 36px; border-radius: 8px; border: 0; cursor: pointer;
    background: rgba(255,255,255,0.1); color: #eef; font-size: 1.2rem; line-height: 1;
  }
  .deck .count { min-width: 2ch; text-align: center; font-size: 1.1rem; }
  .opt { display: flex; align-items: center; gap: .5rem; cursor: pointer; font-size: .9rem; }
  .opt input { accent-color: #2f9e63; }
  .summary { margin: .9rem 0 .4rem; font-size: .9rem; opacity: .9; display: flex; gap: .6rem; align-items: center; flex-wrap: wrap; }
  .warn { color: #ffd27a; font-size: .85rem; }
  .actions { display: flex; gap: .6rem; margin-top: .8rem; }
  .reset {
    padding: .75rem 1rem; border-radius: 9px; cursor: pointer;
    background: transparent; border: 1px solid rgba(255,255,255,0.25); color: #eef; font-size: .95rem;
  }
  .reset:hover { background: rgba(255,255,255,0.08); }
  .start {
    flex: 1; padding: .75rem; border: 0; border-radius: 9px; cursor: pointer;
    background: #2f9e63; color: #fff; font-weight: 600; font-size: 1.02rem;
  }
  .start:hover:not(:disabled) { background: #36b46f; }
  .start:disabled { opacity: .5; cursor: not-allowed; }
  .hint { color: #9fbfb0; margin: .7rem 0 0; font-size: .78rem; line-height: 1.3; }
</style>
