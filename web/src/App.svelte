<script>
  import { onMount, tick } from 'svelte'
  import Card from './lib/Card.svelte'
  import { ensureInlineSprite } from './lib/svg-sprite'
  import { dealToHands, fullDeck } from './lib/deck'
  import { dealFromCenter } from './lib/cards-anim'

  let spriteReady = false
  let allHands = dealToHands(fullDeck(), 4, 5) // 4 joueurs x 5 cartes

  // Références DOM
  let deckEl
  let hand0, hand1, hand2, hand3

  onMount(async () => {
    await ensureInlineSprite('/cards.svg')
    spriteReady = true

    // Attendre que le DOM (cartes & conteneurs) soit présent
    await tick()

    const cardNodes = Array.from(document.querySelectorAll('.card'))
    const targets = [hand0, hand1, hand2, hand3].filter(Boolean)
    dealFromCenter(cardNodes, deckEl, targets, { perPlayer: 5, stagger: 0.06 })
  })
</script>

<style>
  .table { position: relative; min-height: 70vh; }
  .center { position:absolute; left:50%; top:50%; transform:translate(-50%,-50%); }
  .row    { display:flex; gap:12px; flex-wrap:wrap; justify-content:center; margin:12px 0; }
</style>

<main style="padding:16px; font-family:system-ui,Segoe UI,Roboto,Helvetica,Arial">
  <h1 style="margin:0 0 8px">Jeu de cartes — Svelte + GSAP</h1>
  <p style="margin:0 0 16px">Distribution depuis la pile centrale vers 4 mains.</p>

  <div class="table">
    <!-- Pile centrale -->
    <div class="center" bind:this={deckEl}>
      {#if spriteReady}
        <svg width="40" height="56" viewBox="0 0 200 280"><use href="#sym-back"/></svg>
      {/if}
    </div>

    {#if spriteReady}
      <!-- Main 1 -->
      <div class="row" bind:this={hand0}>
        {#each allHands[0] as c}
          <Card {c} width={100} height={140} on:click={() => c.faceUp = !c.faceUp} />
        {/each}
      </div>

      <!-- Main 2 -->
      <div class="row" bind:this={hand1}>
        {#each allHands[1] as c}
          <Card {c} width={100} height={140} on:click={() => c.faceUp = !c.faceUp} />
        {/each}
      </div>

      <!-- Main 3 -->
      <div class="row" bind:this={hand2}>
        {#each allHands[2] as c}
          <Card {c} width={100} height={140} on:click={() => c.faceUp = !c.faceUp} />
        {/each}
      </div>

      <!-- Main 4 -->
      <div class="row" bind:this={hand3}>
        {#each allHands[3] as c}
          <Card {c} width={100} height={140} on:click={() => c.faceUp = !c.faceUp} />
        {/each}
      </div>
    {/if}
  </div>
</main>
