// Géométrie du tapis, en pixels du repère NON réduit — celui que partagent le
// client et le serveur (positions des cartes, sièges des avatars).
//
// Ces valeurs sont DOUBLÉES côté serveur (tableW, tableH, cardW, cardH,
// feltInset dans server/handlers.go) : les garder synchronisées. Un écart ne
// casse rien, mais décale les cartes que le serveur pose lui-même (abandon).

export const TABLE_W = 1440
export const TABLE_H = 800
export const FELT_INSET = 40 // pourtour bois
// 92x128 à l'origine, 110x154 puis 140x196 : sur tablette, le tapis réduit
// rendait les cartes trop petites. Agrandir les cartes par rapport au tapis
// les grossit à l'écran sans demander plus de place.
export const CARD_W = 140
export const CARD_H = 196

// En deçà, les cartes deviennent illisibles : on laisse alors défiler.
export const MIN_SCALE = 0.45

// Hauteur de la bande de la main hors cartes : marges haute et basse (6px
// chacune) et filet supérieur (2px). Cf. `.hand` dans Hand.svelte.
export const HAND_CHROME = 14

// Colonne de gauche (titre, joueur, boutons) : large d'environ deux cartes
// À L'ÉCRAN, donc de 2 × CARD_W × échelle, plus ses marges et son filet.
export const SIDEBAR_CARDS = 2
export const SIDEBAR_CHROME = 24
export function sidebarWidth(scale) {
  return Math.round(SIDEBAR_CARDS * CARD_W * scale) + SIDEBAR_CHROME
}

// fitScale calcule l'échelle d'affichage du tapis pour une zone de jeu de
// width × height pixels, qui contient la colonne de gauche, le tapis, et la
// bande de la main sous les deux.
//
// La main et la colonne de gauche dépendent elles-mêmes de l'échelle (cartes
// de la main à la taille de celles du tapis, colonne large de deux cartes). On
// résout tout d'un seul calcul plutôt que de laisser le tapis mesurer la place
// restante — sinon chaque changement d'échelle redimensionnerait la main ou la
// colonne, qui redimensionneraient le tapis, et ainsi de suite.
//   largeur : SIDEBAR_CHROME + 2·CARD_W·s + TABLE_W·s + marge <= width
//   hauteur : TABLE_H·s + CARD_H·s + HAND_CHROME + marge      <= height
export function fitScale(width, height, pad = 12) {
  const byWidth = (width - SIDEBAR_CHROME - pad) / (TABLE_W + SIDEBAR_CARDS * CARD_W)
  const byHeight = (height - HAND_CHROME - pad) / (TABLE_H + CARD_H)
  const s = Math.min(1, byWidth, byHeight)
  return Math.max(MIN_SCALE, Math.round(s * 1000) / 1000)
}
