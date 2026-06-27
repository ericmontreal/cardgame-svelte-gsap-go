<script>
  import { onMount, afterUpdate } from 'svelte'
  export let c = { faceId: '1_club', faceUp: true }
  export let width = 120
  export let height = 168

  let currentId = c.faceUp ? c.faceId : 'back'
  let backId = 'back'
  let useEl

  function setBoth() {
    if (!useEl) return
    const target = '#sym-' + (c.faceUp ? currentId : backId)
    useEl.setAttribute('href', target)
    useEl.setAttributeNS('http://www.w3.org/1999/xlink', 'xlink:href', target)
  }
  export function setFace(id) { currentId = id; setBoth() }

  onMount(() => {
    const symRoot = document.getElementById('__cards_symbols__')
    if (symRoot) {
      const hit = Array.from(symRoot.querySelectorAll('symbol')).map(s=>s.id).find(id=>/back/i.test(id))
      if (hit) backId = hit.replace(/^sym-/, '')
    }
    setBoth()
  })
  $: currentId = c.faceUp ? c.faceId : backId
  afterUpdate(setBoth)
</script>

<svg class="card" {width} {height} viewBox="0 0 200 280" preserveAspectRatio="xMidYMid meet" style="transform-style:preserve-3d; will-change:transform; overflow:visible">
  <use bind:this={useEl}></use>
</svg>
