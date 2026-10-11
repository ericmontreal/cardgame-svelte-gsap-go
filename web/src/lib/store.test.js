import { describe, it, expect } from 'vitest'
import { receivedText, mergeHandOrder } from './store.js'

describe('receivedText', () => {
  it('annonce un don du sabot à un autre joueur', () => {
    expect(receivedText({ by: 'alice', to: 'bob', self: false, from: 'sabot', count: 1 }))
      .toBe('bob a reçu une carte du sabot, donnée par alice')
  })

  it('accorde le nombre', () => {
    expect(receivedText({ by: 'alice', to: 'bob', self: false, from: 'table', count: 3 }))
      .toBe('bob a reçu 3 cartes du tapis, données par alice')
  })

  it('annonce un don de main à main', () => {
    expect(receivedText({ by: 'alice', to: 'bob', self: false, from: 'hand', count: 1 }))
      .toBe('bob a reçu une carte, donnée par alice depuis sa main')
  })

  it('distingue le joueur qui se sert lui-même', () => {
    expect(receivedText({ by: 'alice', to: 'alice', self: true, from: 'sabot', count: 1 }))
      .toBe('alice a tiré une carte du sabot')
    expect(receivedText({ by: 'alice', to: 'alice', self: true, from: 'table', count: 2 }))
      .toBe('alice a ramassé 2 cartes du tapis')
  })

  it("n'annonce rien sans carte", () => {
    expect(receivedText({ by: 'alice', to: 'bob', from: 'sabot', count: 0 })).toBe('')
    expect(receivedText(undefined)).toBe('')
  })
})

describe('mergeHandOrder', () => {
  const ids = (cs) => cs.map((c) => c.id).join(' ')
  it('ajoute les nouvelles cartes à droite', () => {
    const prev = [{ id: 'c-9' }, { id: 'c-2' }]
    expect(ids(mergeHandOrder(prev, [{ id: 'c-2' }, { id: 'c-5' }, { id: 'c-9' }]))).toBe('c-9 c-2 c-5')
  })
  it('retire les cartes sorties de la main sans déplacer les autres', () => {
    const prev = [{ id: 'a' }, { id: 'b' }, { id: 'c' }]
    expect(ids(mergeHandOrder(prev, [{ id: 'c' }, { id: 'a' }]))).toBe('a c')
  })
  it('prend les données à jour du serveur (carte retournée)', () => {
    const out = mergeHandOrder([{ id: 'a', faceUp: false }], [{ id: 'a', faceUp: true }])
    expect(out[0].faceUp).toBe(true)
  })
})
