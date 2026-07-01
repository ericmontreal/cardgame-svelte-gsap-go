<script>
  // Menu d'initialisation (§4) : sélection par cases à cocher du nombre et du
  // type de jeux de cartes (couleurs, plage de rangs, jokers...).
  import { createEventDispatcher } from 'svelte'

  export let seedHint = ''
  const dispatch = createEventDispatcher()

  // ---- État du formulaire (cases à cocher) ----
  // Couleurs : cases à cocher (au moins une).
  let suits = { club: true, diamond: true, heart: true, spade: true }
  // Bornes de rangs : on coche "de 2 à As" par défaut, l'utilisateur ajuste.
  let fromRank = 1   // 1 = As (affiché "1" dans le sprite)
  let toRank = 13
  // Nombre de jeux : spinner 1..8.
  let deckCount = 1
  // Jokers : radio (aucun / noir / rouge / les deux).
  let jokers = 'none'
  // Mélange à la création du sabot (cases à cocher supplémentaire).
  let shuffle = true

  const rankLabels = {
    1: 'As', 2: '2', 3: '3', 4: '4', 5: '5', 6: '6', 7: '7',
    8: '8', 9: '9', 10: '10', 11: 'V', 12: 'D', 13: 'R',
  }
  const suitGlyph = { club: '♣', diamond: '♦', heart: '♥', spade: '♠' }
  const suitName = { club: 'Trèfle', diamond: 'Carreau', heart: 'Cœur', spade: 'Pique' }

  $: selectedSuits = Object.keys(suits).filter((k) => suits[k])
  $: validSuits = selectedSuits.length > 0
  $: validRange = fromRank <= toRank
  // Estimation du nombre de cartes (aperçu en direct).
  $: estCards =
    (validSuits && validRange)
      ? deckCount * selectedSuits.length * (toRank - fromRank + 1) + jokerCount()
      : 0

  function jokerCount() {
    if (jokers === 'both') return deckCount * 2
    if (jokers === 'black' || jokers === 'red') return deckCount
    return 0
  }

  function start() {
    if (!validSuits || !validRange) return
    dispatch('start', {
      config: {
        deckCount,
        suits: selectedSuits,
        fromRank,
        toRank,
        jokers,
      },
      shuffle,
    })
  }
</script>

<div class="init-wrap">
  <div class="init-card">
    <h2>Préparation du sabot</h2>
    <p class="subtitle">Choisissez la composition du jeu, puis ouvrez la table.</p>

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
      <legend>Plages de rangs</legend>
      <div class="row range">
        <label>de
          <select bind:value={fromRank}>
            {#each Object.keys(rankLabels) as r}
              <option value={Number(r)}>{rankLabels[r]}</option>
            {/each}
          </select>
        </label>
        <span class="arrow">→</span>
        <label>à
          <select bind:value={toRank}>
            {#each Object.keys(rankLabels) as r}
              <option value={Number(r)}>{rankLabels[r]}</option>
            {/each}
          </select>
        </label>
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
      {:else if !validRange}<span class="warn">— plage de rangs invalide</span>{/if}
    </div>

    <button class="start" on:click={start} disabled={!validSuits || !validRange}>
      Ouvrir la table
    </button>
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
  select {
    padding: .4rem .5rem; border-radius: 7px; border: 1px solid rgba(255,255,255,0.15);
    background: rgba(0,0,0,0.3); color: #eef; font-size: .95rem;
  }
  .arrow { opacity: .6; }
  .deck button {
    width: 36px; height: 36px; border-radius: 8px; border: 0; cursor: pointer;
    background: rgba(255,255,255,0.1); color: #eef; font-size: 1.2rem; line-height: 1;
  }
  .deck .count { min-width: 2ch; text-align: center; font-size: 1.1rem; }
  .opt { display: flex; align-items: center; gap: .5rem; cursor: pointer; font-size: .9rem; }
  .opt input { accent-color: #2f9e63; }
  .summary { margin: .9rem 0 .4rem; font-size: .9rem; opacity: .9; display: flex; gap: .6rem; align-items: center; flex-wrap: wrap; }
  .warn { color: #ffd27a; font-size: .85rem; }
  .start {
    margin-top: .8rem; width: 100%; padding: .75rem; border: 0; border-radius: 9px; cursor: pointer;
    background: #2f9e63; color: #fff; font-weight: 600; font-size: 1.02rem;
  }
  .start:hover:not(:disabled) { background: #36b46f; }
  .start:disabled { opacity: .5; cursor: not-allowed; }
  .hint { color: #9fbfb0; margin: .7rem 0 0; font-size: .78rem; line-height: 1.3; }
</style>
