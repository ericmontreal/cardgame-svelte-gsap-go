import { describe, it, expect } from 'vitest'
import { fitScale, sidebarWidth, projectOnSeatRing, placeSeat, TABLE_W, TABLE_H, CARD_H, HAND_CHROME, MIN_SCALE, SEAT_BAND, FELT_INSET, SEAT_GAP } from './geometry.js'

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

describe('projectOnSeatRing', () => {
  const L = FELT_INSET, R = TABLE_W - FELT_INSET, T = FELT_INSET, B = TABLE_H - FELT_INSET
  it('rejoint le côté le plus proche depuis le tapis', () => {
    expect(projectOnSeatRing(300, T + 10)).toEqual({ x: 300, y: T })
    expect(projectOnSeatRing(300, B - 5)).toEqual({ x: 300, y: B })
    expect(projectOnSeatRing(L + 3, 400)).toEqual({ x: L, y: 400 })
    expect(projectOnSeatRing(R - 1, 400)).toEqual({ x: R, y: 400 })
  })
  it('ramène un point extérieur sur le bord', () => {
    expect(projectOnSeatRing(-50, 400)).toEqual({ x: L, y: 400 })
    expect(projectOnSeatRing(700, TABLE_H + 30)).toEqual({ x: 700, y: B })
    expect(projectOnSeatRing(-10, -10)).toEqual({ x: L, y: T })
  })
})

describe('placeSeat', () => {
  const B = TABLE_H - FELT_INSET
  it('garde une place libre telle quelle', () => {
    expect(placeSeat(300, B, [{ x: 720, y: B }])).toEqual({ x: 300, y: B, ok: true })
  })
  it("fait glisser une place trop proche jusqu'à l'écart minimal, du côté du lâcher", () => {
    const p = placeSeat(700, B, [{ x: 720, y: B }])
    expect(p.ok).toBe(true)
    expect(p.y).toBe(B)
    expect(720 - p.x).toBeGreaterThanOrEqual(SEAT_GAP)
    expect(720 - p.x).toBeLessThan(SEAT_GAP + 3)
  })
})
