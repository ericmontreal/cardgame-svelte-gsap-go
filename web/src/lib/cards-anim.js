import { gsap } from 'gsap'

export function dealFromCenter(els, deckEl, targets, { perPlayer=5, stagger=0.06, offsetX=20, offsetY=10 } = {}) {
  const tl = gsap.timeline({ defaults: { duration: 0.45, ease: 'power2.out' } })

  const deckBox = deckEl.getBoundingClientRect()
  const originX = deckBox.left + deckBox.width/2 + offsetX
  const originY = deckBox.top  + deckBox.height/2 + offsetY

  const centers = targets.map(t => {
    const b = t.getBoundingClientRect()
    return { x: b.left + b.width/2, y: b.top + b.height/2 }
  })

  // Positionner toutes les cartes à l’origine décalée
  els.forEach(el => {
    const b = el.getBoundingClientRect()
    const cx = b.left + b.width/2
    const cy = b.top + b.height/2
    gsap.set(el, {
      x: `+=${originX - cx}`,
      y: `+=${originY - cy}`,
      opacity: 0
    })
  })

  // Séquence de distribution
  let idx = 0
  for (let i = 0; i < perPlayer; i++) {
    for (let p = 0; p < targets.length; p++) {
      const el = els[idx++]
      if (!el) break
      const b = el.getBoundingClientRect()
      const cx = b.left + b.width/2
      const cy = b.top + b.height/2
      const dx = centers[p].x - cx
      const dy = centers[p].y - cy
      tl.to(el, { x: `+=${dx}`, y: `+=${dy}`, opacity: 1 }, `+=${stagger}`)
    }
  }
  return tl
}
