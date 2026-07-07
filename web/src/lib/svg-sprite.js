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

  // Cadrage commun des cartes (recto ET verso) sur le contour partagé #base :
  // chaque carte a son propre <use xlink:href="#base" x=".." y=".."> comme
  // ancre, mais un viewBox par carte calculé via getBBox() du groupe entier
  // varie selon le débordement des décors (chiffres/figures pour le recto,
  // dentelle pour le verso) — le dos, moins débordant, se retrouvait donc
  // recadré plus "serré" et s'affichait plus large/zoomé que le recto,
  // décalant sa bordure (cf. lignes blanches/noires visibles au bord).
  // On calcule ici un viewBox de taille IDENTIQUE pour toutes les cartes,
  // ancré sur #base et assez large pour englober le pire débordement
  // observé, afin que le recto et le verso soient mis à l'échelle pareil.
  const baseEl = srcSvg.querySelector('#base')
  let baseFrame = null
  if (baseEl) {
    try {
      const baseBBox = baseEl.getBBox()
      const cardGroups = []
      let padTop = 0, padBottom = 0, padLeft = 0, padRight = 0
      groups.forEach((g) => {
        const useBase = Array.from(g.children).find((c) => {
          if (c.tagName.toLowerCase() !== 'use') return false
          const href = c.getAttribute('href') || c.getAttributeNS('http://www.w3.org/1999/xlink', 'href')
          return href === '#base'
        })
        if (!useBase) return
        try {
          const bbox = g.getBBox()
          const ux = parseFloat(useBase.getAttribute('x') || '0')
          const uy = parseFloat(useBase.getAttribute('y') || '0')
          const baseTop = uy + baseBBox.y
          const baseBottom = baseTop + baseBBox.height
          const baseLeft = ux + baseBBox.x
          const baseRight = baseLeft + baseBBox.width
          padTop = Math.max(padTop, baseTop - bbox.y)
          padBottom = Math.max(padBottom, (bbox.y + bbox.height) - baseBottom)
          padLeft = Math.max(padLeft, baseLeft - bbox.x)
          padRight = Math.max(padRight, (bbox.x + bbox.width) - baseRight)
          cardGroups.push({ g, ux, uy })
        } catch (e) {
          // ignoré : ce groupe sera traité par le repli générique ci-dessous
        }
      })
      // Marge de sécurité pour le débord du trait du contour (#base a un
      // stroke-width de 2.5, exclu du getBBox) et pour éviter tout rognage
      // par arrondi flottant.
      const STROKE_PAD = 2
      padTop += STROKE_PAD; padBottom += STROKE_PAD; padLeft += STROKE_PAD; padRight += STROKE_PAD
      baseFrame = {
        cardGroups,
        width: baseBBox.width + padLeft + padRight,
        height: baseBBox.height + padTop + padBottom,
        left: baseBBox.x - padLeft,
        top: baseBBox.y - padTop,
      }
    } catch (e) {
      if (VERBOSE && e) console.warn('[cards] cadrage commun impossible, repli par carte:', e.message)
    }
  }
  const framedIds = new Set((baseFrame ? baseFrame.cardGroups : []).map(({ g }) => g.id))

  let made = 0
  let skipped = 0
  if (baseFrame) {
    baseFrame.cardGroups.forEach(({ g, ux, uy }) => {
      const symbol = document.createElementNS(NS, 'symbol')
      symbol.setAttribute('id', 'sym-' + g.id)
      symbol.setAttribute('viewBox', `${ux + baseFrame.left} ${uy + baseFrame.top} ${baseFrame.width} ${baseFrame.height}`)
      symbol.appendChild(g.cloneNode(true))
      symSvg.appendChild(symbol)
      made++
    })
  }
  groups.forEach((g) => {
    if (framedIds.has(g.id)) return
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
