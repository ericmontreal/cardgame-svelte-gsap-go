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
    // Cas spécial : la zone "table" renvoie des coordonnées dans le repère du
    // tapis NON réduit (celui du serveur). La table peut être réduite pour
    // tenir à l'écran (Table.svelte) : son échelle, exposée par data-scale,
    // convertit les pixels d'écran. Le caller peut passer tableRect ; à
    // défaut (drop depuis la main), on prend le rect de l'élément trouvé.
    if (t === TARGETS.TABLE) {
      const tr = opts.tableRect ?? rect
      const s = Number(el.getAttribute('data-scale')) || 1
      return {
        target: TARGETS.TABLE,
        ownerId: '',
        x: (clientX - tr.left) / s,
        y: (clientY - tr.top) / s,
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
// relatives au tapis, ou null si le relâchement est hors table. `scale` :
// échelle d'affichage du tapis (cf. Table.svelte).
export function tableCoords(clientX, clientY, tableRect, scale = 1) {
  if (!tableRect) return null
  return {
    x: (clientX - tableRect.left) / scale,
    y: (clientY - tableRect.top) / scale,
  }
}
