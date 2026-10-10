import { describe, it, expect } from 'vitest'
import { receivedText } from './store.js'

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
