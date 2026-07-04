import { defineConfig } from 'vitest/config'

export default defineConfig({
  test: {
    // Les tests portent sur de la logique pure (deck.js) : environnement node.
    environment: 'node',
    include: ['src/**/*.test.js'],
  },
})
