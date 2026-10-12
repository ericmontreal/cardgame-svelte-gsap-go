import { describe, it, expect } from 'vitest'
import { fitScale, sidebarWidth, TABLE_W, TABLE_H, CARD_H, HAND_CHROME, MIN_SCALE, SEAT_BAND } from './geometry.js'

describe('fitScale', () => {
  it('ne grossit jamais le tapis', () => {
    expect(fitScale(4000, 3000)).toBe(1)
  })

  it('fait tenir la colonne, le tapis ET la main dans la zone de jeu', () => {
    for (const [w, h] of [[1020, 720], [920, 740], [1660, 960], [1200, 1100]]) {
      const s = fitScale(w, h)
      expect(sidebarWidth(s) + TABLE_W * s).toBeLessThanOrEqual(w)
      expect((TABLE_H + 2 * SEAT_BAND) * s + CARD_H * s + HAND_CHROME).toBeLessThanOrEqual(h)
    }
  })

  it('plafonne la réduction pour garder des cartes lisibles', () => {
    expect(fitScale(200, 150)).toBe(MIN_SCALE)
  })
})
