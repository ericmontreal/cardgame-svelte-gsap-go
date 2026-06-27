# Squelette Go (WebSocket) + Svelte + GSAP — Jeu de cartes multi‑joueurs

Ce projet expose :
- **Serveur Go** minimal en WebSocket (rooms, broadcast).
- **Front Svelte** (Vite) avec **GSAP** pour animer des cartes **SVG** via `<use href="#…">`.

## Arborescence
```
cardgame-svelte-gsap-go/
├─ server/
│  ├─ main.go
│  └─ go.mod
└─ web/
   ├─ public/
   │  └─ cards.svg        # Remplacez par le sprite officiel (Svg-cards-2.0.svg)
   ├─ src/
   │  ├─ lib/
   │  │  ├─ Card.svelte
   │  │  ├─ cards-anim.js
   │  │  ├─ deck.js
   │  │  └─ svg-sprite.js
   │  ├─ App.svelte
   │  └─ main.js
   ├─ index.html
   ├─ package.json
   └─ vite.config.js
```

## Prérequis
- **Go 1.21+**
- **Node 18+**

## Démarrage
1) **Front** :  
```bash
cd web
npm install
npm run dev
```
2) **Serveur Go** :  
```bash
cd ../server
go mod tidy
go run .
```
3) Ouvrez http://localhost:5173 — le front tente `ws://localhost:8080/ws?room=lobby`.

### Important — Sprite SVG
Remplacez `web/public/cards.svg` par le fichier **Svg-cards-2.0.svg** renommé en `cards.svg`.  
Le code s’attend à des IDs du style `spade_king`, `heart_1`, etc., et `back` pour le dos.  
Le placeholder fourni ne contient que quelques symboles — il sert juste à voir l’appli démarrer.

## À faire côté jeu
- Logique d’état autoritaire côté serveur (distribution, tours, règles).
- Messages structurés (join/leave/action/state).
- Anti-triche et persistance.
- Gestion des rooms multiples et reconnexion.
