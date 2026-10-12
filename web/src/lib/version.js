// Détection d'une nouvelle version de l'interface mise en ligne.
//
// Une mise en ligne remplace les fichiers du site, mais un onglet déjà ouvert
// garde l'ancien code en mémoire : se reconnecter (même après un redémarrage
// du serveur) ne recharge pas la page. Le joueur jouait alors avec l'ancienne
// interface contre le nouveau serveur, sans le savoir (constaté le 2026-10-12 :
// correctif du sabot en ligne, onglet resté sur l'ancien dessin).
//
// En production, index.html charge un script au nom haché
// (/assets/index-<empreinte>.js) qui change à chaque construction. On compare
// celui de la page en cours à celui que sert le site maintenant.

const ENTRY_RE = /src="(\/assets\/index-[^"]+\.js)"/

// Script d'entrée de la page en cours, ou '' hors production (serveur Vite :
// /src/main.js, rechargé à chaud de toute façon).
function loadedEntry() {
  for (const s of document.querySelectorAll('script[type="module"][src]')) {
    const src = s.getAttribute('src')
    if (src && src.startsWith('/assets/')) return src
  }
  return ''
}

// Vrai si le site sert un autre script d'entrée que celui de la page en cours.
// Faux en cas de doute (réseau indisponible, page inattendue).
export async function newVersionAvailable() {
  const current = loadedEntry()
  if (!current) return false
  try {
    const res = await fetch('/', { cache: 'no-store' })
    if (!res.ok) return false
    const m = (await res.text()).match(ENTRY_RE)
    return !!m && m[1] !== current
  } catch {
    return false
  }
}

// Recharge la page si une nouvelle version est en ligne. La session vit dans
// sessionStorage, propre à l'onglet : elle survit au rechargement, et le
// joueur retrouve sa table telle que le serveur la garde.
let reloading = false
export async function reloadIfNewVersion() {
  if (reloading) return true
  if (await newVersionAvailable()) {
    reloading = true
    location.reload()
    return true
  }
  return false
}
