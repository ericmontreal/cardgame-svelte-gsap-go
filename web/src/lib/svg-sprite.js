// web/src/lib/svg-sprite.js
const NS = 'http://www.w3.org/2000/svg'

// En mode dev, on journalise les avertissements utiles. Passez à false (ou
// définissez import.meta.env.PROD via Vite) pour les couper en production.
const VERBOSE = false

export async function ensureInlineSprite(path = '/cards.svg') {
  if (document.getElementById('__cards_symbols__')) return

  const res = await fetch(path)
  if (!res.ok) {
    console.warn(`[cards] sprite introuvable: ${path} (${res.status})`)
    return
  }
  const text = await res.text()

  // Garder "rendu" (pas display:none) pour que getBBox fonctionne
  const holder = document.createElement('div')
  holder.id = '__cards_holder__'
  Object.assign(holder.style, {
    position: 'absolute', width: '0', height: '0', overflow: 'hidden',
    left: '-9999px', top: '-9999px',
  })
  holder.innerHTML = text
  document.body.appendChild(holder)

  const srcSvg = holder.querySelector('svg')
  if (!srcSvg) {
    console.warn('[cards] Pas de <svg> root dans', path)
    holder.remove()
    return
  }

  const symSvg = document.createElementNS(NS, 'svg')
  symSvg.setAttribute('xmlns', NS)
  symSvg.setAttribute('id', '__cards_symbols__')
  Object.assign(symSvg.style, {
    position: 'absolute', width: '0', height: '0', overflow: 'hidden',
    left: '-9999px', top: '-9999px',
  })

  const defs = srcSvg.querySelector('defs')
  if (defs) symSvg.appendChild(defs.cloneNode(true))

  const groups = srcSvg.querySelectorAll('g[id]')
  let made = 0
  let skipped = 0
  groups.forEach((g) => {
    try {
      const bbox = g.getBBox()
      if (!bbox || bbox.width === 0 || bbox.height === 0) {
        skipped++
        return
      }
      const symbol = document.createElementNS(NS, 'symbol')
      symbol.setAttribute('id', 'sym-' + g.id)
      symbol.setAttribute('viewBox', `${bbox.x} ${bbox.y} ${bbox.width} ${bbox.height}`)
      symbol.appendChild(g.cloneNode(true))
      symSvg.appendChild(symbol)
      made++
    } catch (e) {
      // getBBox peut lever si le groupe n'est pas rendu ; on ignore silencieusement
      // sauf en mode verbeux (utile pour diagnostiquer un sprite problématique).
      skipped++
      if (VERBOSE && e) console.warn('[cards] symbole ignoré:', g.id, e.message)
    }
  })
  document.body.appendChild(symSvg)
  holder.remove()

  if (VERBOSE) {
    console.log(`[cards] ${made} symbole(s) créé(s)${skipped ? `, ${skipped} ignoré(s)` : ''}`)
  }
}
