<script>
  // Menu d'initialisation (§4) : sélection par cases à cocher du nombre et du
  // type de jeux de cartes (valeurs, couleurs, jokers...). Inspiré de
  // web/config_jeu.html (cases à cocher par valeur de carte plutôt qu'une
  // plage de-à). Affiché en début de session et à chaque nouvelle partie ;
  // la configuration choisie est conservée (localStorage) tant que le bouton
  // "Réinitialiser" n'est pas cliqué.
  import { createEventDispatcher } from 'svelte'
  import { loadDeckConfig, saveDeckConfig, clearDeckConfig } from './store.js'

  // Une partie est en cours (le joueur vient de « Nouvelle partie ») : on lui
  // offre de revenir à la table sans rien changer, et on l'avertit de ce que
  // « Ouvrir la table » fera aux autres.
  export let gameInProgress = false

  const dispatch = createEventDispatcher()

  function cancel() {
    if (gameInProgress) dispatch('cancel')
  }
  function onKey(e) {
    if (e.key === 'Escape') cancel()
  }

  // ---- Valeurs par défaut (jeu classique 52 cartes) ----
  const defaultSuits = { club: true, diamond: true, heart: true, spade: true }
  const defaultRanks = {
    1: true, 2: true, 3: true, 4: true, 5: true, 6: true, 7: true,
    8: true, 9: true, 10: true, 11: true, 12: true, 13: true,
  }
  const defaultDeckCount = 1
  const defaultJokers = 'none'
  const defaultShuffle = true
  const defaultSnapEnabled = true
  const defaultHandFaceUp = true

  // ---- État du formulaire, pré-rempli depuis la config persistée ----
  const persisted = loadDeckConfig()
  let suits = { ...defaultSuits, ...(persisted?.suits ?? {}) }
  let ranksChecked = { ...defaultRanks, ...(persisted?.ranks ?? {}) }
  let deckCount = persisted?.deckCount ?? defaultDeckCount
  let jokers = persisted?.jokers ?? defaultJokers
  let shuffle = persisted?.shuffle ?? defaultShuffle
  let snapEnabled = persisted?.snapEnabled ?? defaultSnapEnabled
  let handFaceUp = persisted?.handFaceUp ?? defaultHandFaceUp

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

  // Valeurs réparties sur deux lignes : les cartes numérotées, puis les
  // figures et l'As.
  const numberRanks = [2, 3, 4, 5, 6, 7, 8, 9, 10]
  const faceRanks = [11, 12, 13, 1]
  $: allRanks = selectedRanks.length === rankOrder.length

  // Cases "Aucune" et "Toutes" : décochent ou cochent toutes les valeurs d'un
  // coup, plutôt que les 13 cases une à une.
  //
  // Ce sont des actions, pas des réglages : leur état coché se déduit de la
  // sélection. Après un clic, on réaligne la case sur cet état — sinon, un clic
  // sur « Aucune » alors que rien n'est coché la décochait à l'écran, l'état
  // calculé n'ayant pas changé et Svelte ne la redessinant pas.
  function setAllRanks(value, e) {
    rankOrder.forEach((r) => (ranksChecked[r] = value))
    ranksChecked = { ...ranksChecked }
    e.currentTarget.checked = true
  }
  // Estimation du nombre de cartes (aperçu en direct), même règle que le
  // serveur (deck.go, Normalize) : jeux × (couleurs × valeurs + jokers).
  //
  // La composition est valable dès qu'elle produit une carte. Exiger une
  // couleur et une valeur refusait un sabot de jokers seuls, pourtant
  // légitime ; le serveur, lui, accepte désormais une liste de valeurs vide.
  //
  // `jokers` et `deckCount` sont cités ici même, et non seulement dans une
  // fonction appelée : Svelte ne recalcule une déclaration réactive que pour
  // les variables qu'elle nomme. Avec l'ancien jokerCount(), changer de
  // jokers ne mettait pas l'aperçu à jour.
  $: jokersPerDeck = jokers === 'both' ? 2 : (jokers === 'black' || jokers === 'red') ? 1 : 0
  $: estCards = deckCount * (selectedSuits.length * selectedRanks.length + jokersPerDeck)
  $: canStart = estCards > 0

  function start() {
    if (!canStart) return
    saveDeckConfig({ suits: { ...suits }, ranks: { ...ranksChecked }, deckCount, jokers, shuffle, snapEnabled, handFaceUp })
    dispatch('start', {
      config: {
        deckCount,
        suits: selectedSuits,
        ranks: selectedRanks,
        jokers,
      },
      shuffle,
      snapEnabled,
      handFaceUp,
    })
  }

  function reset() {
    clearDeckConfig()
    suits = { ...defaultSuits }
    ranksChecked = { ...defaultRanks }
    deckCount = defaultDeckCount
    jokers = defaultJokers
    shuffle = defaultShuffle
    snapEnabled = defaultSnapEnabled
    handFaceUp = defaultHandFaceUp
  }
</script>

<svelte:window on:keydown={onKey} />

<div class="init-wrap">
  <div class="init-card">
    <h2>Configuration du jeu</h2>
    {#if gameInProgress}
      <p class="in-progress">
        Une partie est en cours. <b>Ouvrir la table</b> remplacera le sabot et
        videra la main de <b>tous les joueurs</b>. Pour la poursuivre, choisissez
        <b>Revenir à la partie</b>.
      </p>
    {/if}

    <!-- Rubriques sur deux colonnes : empilées sur une seule, elles rendaient
         la fenêtre plus haute qu'une tablette en paysage, et la ligne des
         boutons tombait sous le bord de l'écran. -->
    <div class="fields">
    <fieldset class="wide">
      <legend>Valeurs des cartes</legend>
      <div class="row ranks">
        <label class="chip all" class:checked={!validRanks}>
          <input type="checkbox" checked={!validRanks} on:change={(e) => setAllRanks(false, e)} />
          <span>Aucune</span>
        </label>
        {#each numberRanks as r}
          <label class="chip" class:checked={ranksChecked[r]}>
            <input type="checkbox" bind:checked={ranksChecked[r]} />
            <span>{rankLabel[r]}</span>
          </label>
        {/each}
      </div>
      <div class="row ranks">
        <label class="chip all" class:checked={allRanks}>
          <input type="checkbox" checked={allRanks} on:change={(e) => setAllRanks(true, e)} />
          <span>Toutes</span>
        </label>
        {#each faceRanks as r}
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
      <legend>Jokers</legend>
      <div class="row jokers">
        {#each [['none', 'Aucun'], ['red', 'Le rouge'], ['black', 'Le noir'], ['both', 'Tous']] as [val, lbl]}
          <label class="chip" class:checked={jokers === val}>
            <input type="radio" name="jokers" value={val} bind:group={jokers} />
            <span>{lbl}</span>
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
      <legend>Options</legend>
      <label class="opt">
        <input type="checkbox" bind:checked={shuffle} />
        Mélanger le sabot à la création
      </label>
      <label class="opt">
        <input type="checkbox" bind:checked={snapEnabled} />
        Aimanter les cartes proches (placement côte à côte)
      </label>
      <!-- Réglage de la partie, pour tous les joueurs : une carte qui entre
           dans une main y est mise face visible (le joueur peut toujours la
           retourner ensuite d'un double-clic). -->
      <label class="opt">
        <input type="checkbox" bind:checked={handFaceUp} />
        Dans la main, placer systématiquement les cartes face visible
      </label>
    </fieldset>
    </div>

    <!-- Aperçu et boutons sur une même ligne : une ligne de moins en hauteur. -->
    <div class="actions">
      <div class="summary">
        <span>Aperçu : <strong>{estCards}</strong> carte{estCards > 1 ? 's' : ''}</span>
        {#if !canStart}<span class="warn">— {!validRanks ? 'sélectionnez au moins une valeur' : 'sélectionnez au moins une couleur'}, ou des jokers</span>{/if}
      </div>
      {#if gameInProgress}
        <button class="back" type="button" on:click={cancel} title="Retrouver la partie en cours, sans rien changer (Échap)">Revenir à la partie</button>
      {/if}
      <button class="reset" type="button" on:click={reset}>Réinitialiser</button>
      <button class="start" on:click={start} disabled={!canStart}>
        Ouvrir la table
      </button>
    </div>
  </div>
</div>

<style>
  .init-wrap {
    /* 100dvh : zone réellement visible sous la barre d'outils des navigateurs
       mobiles (cf. App.svelte). Le padding garde un peu d'air autour du cadre,
       et la page défile encore si l'écran est vraiment trop bas. */
    min-height: 100vh; min-height: 100dvh; box-sizing: border-box; padding: 12px;
    display: grid; place-items: center;
    background: radial-gradient(circle at 50% 30%, #1c6e4b 0%, #0d3a26 70%, #062016 100%);
    font-family: system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif;
  }
  .init-card {
    /* Largeur réglée sur la ligne la plus longue, celle des valeurs « Aucune,
       2 … 10 », pour qu'elle tienne sans retour à la ligne : 690px de cases
       mesurés sous Windows, plus une marge pour les polices d'Android. */
    width: min(790px, 96vw);
    box-sizing: border-box;
    background: rgba(10, 30, 22, 0.92);
    color: #eef;
    padding: 1.2rem 1.6rem 1.1rem;
    border-radius: 14px;
    border: 1px solid rgba(255,255,255,0.08);
    box-shadow: 0 20px 60px rgba(0,0,0,0.45);
  }
  h2 { margin: 0 0 .6rem; font-size: 1.35rem; }

  .fields { display: grid; grid-template-columns: 1fr 1fr; column-gap: .9rem; }
  .fields .wide { grid-column: 1 / -1; }
  /* En portrait étroit (téléphone), retour à une seule colonne. */
  @media (max-width: 700px) { .fields { grid-template-columns: 1fr; } }
  fieldset { border: 1px solid rgba(255,255,255,0.1); border-radius: 10px; padding: .6rem .9rem; margin: .4rem 0; min-width: 0; }
  legend { padding: 0 .4rem; font-size: .8rem; opacity: .8; text-transform: uppercase; letter-spacing: .04em; }
  .row { display: flex; gap: .6rem; align-items: center; flex-wrap: wrap; }
  .row.ranks { gap: .45rem; }
  .row.ranks .chip { padding: .4rem .6rem; }
  .row.ranks + .row.ranks { margin-top: .5rem; }
  /* « Aucune » et « Toutes » : même largeur, pour que « 2 » et « Valets »
     commencent à la même abscisse, et un écart un peu plus grand après elles,
     qui les met légèrement à part des valeurs. */
  .chip.all { font-weight: 600; box-sizing: border-box; min-width: 6.4rem; margin-right: .55rem; }
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
  .opt + .opt { margin-top: .5rem; }
  .opt input { accent-color: #2f9e63; }
  .summary { flex: 1; min-width: 0; font-size: .9rem; opacity: .9; display: flex; gap: .2rem .6rem; align-items: center; flex-wrap: wrap; }
  .warn { color: #ffd27a; font-size: .85rem; }
  .actions { display: flex; gap: .6rem; align-items: center; margin-top: .7rem; }
  .reset {
    padding: .75rem 1rem; border-radius: 9px; cursor: pointer;
    background: transparent; border: 1px solid rgba(255,255,255,0.25); color: #eef; font-size: .95rem;
  }
  .reset:hover { background: rgba(255,255,255,0.08); }
  .back {
    padding: .75rem 1rem; border-radius: 9px; cursor: pointer;
    background: transparent; border: 1px solid #ffd27a; color: #ffd27a; font-size: .95rem;
  }
  .back:hover { background: rgba(255,210,122,0.12); }
  .in-progress {
    margin: 0 0 .6rem; padding: .5rem .8rem; border-radius: 8px;
    background: rgba(255,210,122,0.1); border-left: 3px solid #ffd27a;
    font-size: .85rem; line-height: 1.4;
  }
  .in-progress b { color: #ffd27a; }
  .start {
    flex: none; padding: .75rem 1.8rem; border: 0; border-radius: 9px; cursor: pointer;
    background: #2f9e63; color: #fff; font-weight: 600; font-size: 1.02rem;
  }
  .start:hover:not(:disabled) { background: #36b46f; }
  .start:disabled { opacity: .5; cursor: not-allowed; }
</style>
