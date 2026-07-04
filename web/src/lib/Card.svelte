<script>
  import { onMount, afterUpdate, createEventDispatcher } from 'svelte'

  // État de carte transmis par le parent (en lecture seule côté composant).
  export let c = { faceId: '1_club', faceUp: true }
  export let width = 120
  export let height = 168

  const dispatch = createEventDispatcher()

  // L'identifiant courant du symbole à afficher. Calculé de façon réactive à
  // partir de l'état `c`, et mis à jour dans le DOM via setBoth().
  let currentId = c.faceUp ? c.faceId : 'back'
  let backId = 'back'
  let useEl

  // Permet au parent de récupérer ce nœud DOM pour les animations GSAP, sans
  // recourir à un querySelectorAll global fragile.
  function getEl() {
    return useEl?.closest('svg')
  }
  export { getEl }

  function setBoth() {
    if (!useEl) return
    const target = '#sym-' + (c.faceUp ? currentId : backId)
    useEl.setAttribute('href', target)
    useEl.setAttributeNS('http://www.w3.org/1999/xlink', 'xlink:href', target)
  }
  export function setFace(id) { currentId = id; setBoth() }

  // Le parent est seul responsable de l'état `c` : on émet un événement au lieu
  // de muter la prop (flux de données unidirectionnel).
  function flip() { dispatch('flip') }

  onMount(() => {
    const symRoot = document.getElementById('__cards_symbols__')
    if (symRoot) {
      const hit = Array.from(symRoot.querySelectorAll('symbol')).map(s => s.id).find(id => /back/i.test(id))
      if (hit) backId = hit.replace(/^sym-/, '')
    }
    setBoth()
  })
  $: currentId = c.faceUp ? c.faceId : backId
  afterUpdate(setBoth)
</script>

<svg class="card" {width} {height} viewBox="0 0 200 280" preserveAspectRatio="xMidYMid meet" style="transform-style:preserve-3d; will-change:transform; overflow:visible" on:click={flip}>
  <use bind:this={useEl}></use>
</svg>
