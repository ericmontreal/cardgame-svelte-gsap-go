// Utilitaire de hit-test pour le drag-and-drop : détermine la cible de drop
// (table / sabot / avatar / main) à partir de la position de relâchement.
//
// Les éléments candidats portent un attribut data-drop="<target>".
// On renvoie { target, ownerId, x, y } où ownerId n'est rempli que pour un
// avatar (data-owner="<userId>") ou la main (data-owner="self").

export const TARGETS = {
  TABLE: 'table',
  SABOT: 'sabot',
  AVATAR: 'avatar',
  HAND: 'hand',
}

// dropAt inspecte les éléments sous le point (clientX, clientY) et renvoie la
// meilleure cible. Priority order : avatar > sabot > main > table (la table
// est le fallback par défaut, comme une vraie surface).
export function dropAt(clientX, clientY, opts = {}) {
  const els = document.elementsFromPoint(clientX, clientY)
  for (const el of els) {
    const t = el.getAttribute && el.getAttribute('data-drop')
    if (!t) continue
    const owner = el.getAttribute('data-owner') || ''
    // Rect de l'élément cible pour convertir en coordonnées locales si besoin.
    const rect = el.getBoundingClientRect()
    const local = { x: clientX - rect.left, y: clientY - rect.top }
    // Cas spécial : la zone "table" doit utiliser le repère de la table (le
    // caller passe tableRect pour calculer les coordonnées tapis).
    if (t === TARGETS.TABLE && opts.tableRect) {
      const tr = opts.tableRect
      return {
        target: TARGETS.TABLE,
        ownerId: '',
        x: clientX - tr.left,
        y: clientY - tr.top,
        local,
      }
    }
    return { target: t, ownerId: owner, x: local.x, y: local.y, local }
  }
  // Aucun drop target : on considère qu'on relâche "dans le vide" -> aucun
  // transfert (la carte reste où elle était). Le caller décide.
  return null
}

// Hit-test spécialisé pour la table uniquement : renvoie les coordonnées
// relatives au tapis, ou null si le relâchement est hors table.
export function tableCoords(clientX, clientY, tableRect) {
  if (!tableRect) return null
  return {
    x: clientX - tableRect.left,
    y: clientY - tableRect.top,
  }
}
