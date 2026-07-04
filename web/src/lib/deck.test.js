import { describe, it, expect } from 'vitest'
import { suits, ranks, faceIdOf, fullDeck, shuffle, dealToHands } from './deck'

describe('faceIdOf', () => {
  it('produit le format "rank_suit"', () => {
    expect(faceIdOf('club', 1)).toBe('1_club')
    expect(faceIdOf('heart', 'queen')).toBe('queen_heart')
    expect(faceIdOf('spade', 'king')).toBe('king_spade')
  })
})

describe('fullDeck', () => {
  it('contient 52 cartes (4 couleurs x 13 rangs)', () => {
    expect(fullDeck()).toHaveLength(52)
  })

  it('ne contient aucun faceId en doublon', () => {
    const ids = fullDeck().map((c) => c.faceId)
    expect(new Set(ids).size).toBe(ids.length)
  })

  it('couvre toutes les combinaisons couleur x rang', () => {
    const ids = new Set(fullDeck().map((c) => c.faceId))
    for (const s of suits) {
      for (const r of ranks) {
        expect(ids.has(faceIdOf(s, r))).toBe(true)
      }
    }
  })
})

describe('shuffle (Fisher-Yates)', () => {
  it('ne mute pas le tableau source', () => {
    const src = [1, 2, 3, 4, 5]
    const copy = [...src]
    shuffle(src)
    expect(src).toEqual(copy)
  })

  it('conserve exactement les mêmes éléments (mêmes multiplicités)', () => {
    const src = fullDeck()
    const shuffled = shuffle(src)
    expect(shuffled).toHaveLength(src.length)
    // Triés, les deux jeux doivent être identiques.
    const sortFn = (a, b) => (a.faceId < b.faceId ? -1 : a.faceId > b.faceId ? 1 : 0)
    expect([...shuffled].sort(sortFn)).toEqual([...src].sort(sortFn))
  })

  it('produit une distribution variée (non constante)', () => {
    // Sur plusieurs tirages, le résultat ne doit pas être toujours identique
    // à l'entrée triée. On vérifie qu'au moins un tirage sur 10 est mélangé.
    const src = Array.from({ length: 52 }, (_, i) => i)
    let anyShuffled = false
    for (let i = 0; i < 10; i++) {
      const out = shuffle(src)
      if (out.some((v, idx) => v !== idx)) {
        anyShuffled = true
        break
      }
    }
    expect(anyShuffled).toBe(true)
  })
})

describe('dealToHands', () => {
  it('distribue le bon nombre de cartes par joueur', () => {
    const hands = dealToHands(fullDeck(), 4, 5)
    expect(hands).toHaveLength(4)
    hands.forEach((hand) => expect(hand).toHaveLength(5))
  })

  it('ne distribue jamais la même carte deux fois', () => {
    const hands = dealToHands(fullDeck(), 4, 5)
    const ids = hands.flat().map((c) => c.faceId)
    expect(new Set(ids).size).toBe(ids.length)
  })

  it('gère des paramètres de distribution personnalisés', () => {
    const hands = dealToHands(fullDeck(), 2, 7)
    expect(hands).toHaveLength(2)
    hands.forEach((hand) => expect(hand).toHaveLength(7))
  })
})
