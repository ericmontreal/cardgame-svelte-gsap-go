// Géométrie du tapis, en pixels du repère NON réduit — celui que partagent le
// client et le serveur (positions des cartes, sièges des avatars).
//
// Ces valeurs sont DOUBLÉES côté serveur (tableW, tableH, cardW, cardH,
// feltInset dans server/handlers.go) : les garder synchronisées. Un écart ne
// casse rien, mais décale les cartes que le serveur pose lui-même (abandon).

export const TABLE_W = 1440
export const TABLE_H = 800
export const FELT_INSET = 40 // pourtour bois
export const CARD_W = 110
export const CARD_H = 154

// En deçà, les cartes deviennent illisibles : on laisse alors défiler.
export const MIN_SCALE = 0.45

// Hauteur de la bande de la main hors cartes : marges haute et basse (6px
// chacune) et filet supérieur (2px). Cf. `.hand` dans Hand.svelte.
export const HAND_CHROME = 14

// fitScale calcule l'échelle d'affichage du tapis pour une zone de jeu de
// width × height pixels, qui contient le tapis ET la bande de la main sous lui.
//
// Les cartes de la main sont affichées à la même taille que celles du tapis
// (CARD_H × échelle) : la hauteur de la main dépend donc de l'échelle. On la
// résout ici d'un seul calcul plutôt que de laisser le tapis mesurer la place
// restante — sinon chaque changement d'échelle redimensionnerait la main, qui
// redimensionnerait le tapis, et ainsi de suite.
//   hauteur : TABLE_H·s + CARD_H·s + HAND_CHROME + marge <= height
export function fitScale(width, height, pad = 12) {
  const byWidth = (width - pad) / TABLE_W
  const byHeight = (height - HAND_CHROME - pad) / (TABLE_H + CARD_H)
  const s = Math.min(1, byWidth, byHeight)
  return Math.max(MIN_SCALE, Math.round(s * 1000) / 1000)
}
