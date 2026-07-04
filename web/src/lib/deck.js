export const suits = ['club','diamond','heart','spade']
// Votre sprite expose visiblement 1..10 puis figures
export const ranks = [1,2,3,4,5,6,7,8,9,10,'jack','queen','king']

// IMPORTANT: ordre rang_suit → "1_club", "queen_heart", etc.
export function faceIdOf(suit, rank) {
  return `${rank}_${suit}`
}

// Jeu complet
export function fullDeck() {
  return suits.flatMap(s =>
    ranks.map(r => ({
      suit: s,
      rank: r,
      faceId: faceIdOf(s, r),
      faceUp: true
    }))
  )
}

// Mélange non biaisé (Fisher-Yates). `Array.sort(() => 0.5 - Math.random())`
// produit une distribution inégale et ne convient pas à un jeu de cartes.
export function shuffle(arr) {
  const a = [...arr]
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1))
    ;[a[i], a[j]] = [a[j], a[i]]
  }
  return a
}

// Distribution simple
export function dealToHands(deck, players = 4, perPlayer = 5) {
  const shuffled = shuffle(deck)
  const hands = Array.from({ length: players }, () => [])
  for (let i = 0; i < perPlayer; i++) {
    for (let p = 0; p < players; p++) {
      hands[p].push(shuffled.pop())
    }
  }
  return hands
}
