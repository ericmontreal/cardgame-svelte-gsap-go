import { describe, it, expect } from 'vitest'
import { fitScale, TABLE_W, TABLE_H, CARD_H, HAND_CHROME, MIN_SCALE } from './geometry.js'

describe('fitScale', () => {
  it('ne grossit jamais le tapis', () => {
    expect(fitScale(4000, 3000)).toBe(1)
  })

  it('fait tenir le tapis ET la main dans la zone de jeu', () => {
    for (const [w, h] of [[1008, 675], [906, 694], [1600, 900], [700, 1100]]) {
      const s = fitScale(w, h)
      expect(TABLE_W * s).toBeLessThanOrEqual(w)
      expect(TABLE_H * s + CARD_H * s + HAND_CHROME).toBeLessThanOrEqual(h)
    }
  })

  it('plafonne la réduction pour garder des cartes lisibles', () => {
    expect(fitScale(200, 150)).toBe(MIN_SCALE)
  })
})
