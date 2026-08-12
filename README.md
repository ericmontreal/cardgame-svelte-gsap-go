# Squelette Go (WebSocket) + Svelte + GSAP — Jeu de cartes multi‑joueurs

Ce projet expose :
- **Serveur Go** minimal en WebSocket (rooms, broadcast), origine restreinte et
  heartbeat natif.
- **Front Svelte** (Vite) avec **GSAP** pour animer des cartes **SVG** via
  `<use href="#…">`, connecté au serveur via un client WS embarquant ping/pong
  et reconnexion automatique.

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
   │  │  ├─ svg-sprite.js
   │  │  └─ ws-client.js
   │  ├─ App.svelte
   │  └─ main.js
   ├─ index.html
   ├─ package.json
   └─ vite.config.js
```

## Prérequis
- **Go 1.25+**
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
3) Ouvrez http://localhost:5173 — le front se connecte à `ws://localhost:8080/ws?room=lobby`.

### Synchronisation multi-clients
À la connexion, le front s'abonne à la room `lobby`. Le **premier client** arrivé
qui ne reçoit pas de deal sous ~1,2 s génère la distribution (mélange Fisher-Yates)
et la diffuse aux autres via le broadcast du serveur. Les clients suivants
**appliquent** ce deal reçu (sans redistribuer eux-mêmes). Un statut de connexion
(connecting / open / reconnecting / error) est affiché en haut de la page.

> Cette synchro est **coopérative** (état généré côté client puis relayé), non
> autoritaire. Voir « À faire » pour un véritable état serveur.

## Tests
**Serveur (Go)** — `server/hub_test.go` couvre le hub (join/leave, broadcast,
isolation par room, concurrence, unicité des IDs).
```bash
cd server
go test ./...
```
**Front (Vitest)** — `web/src/lib/deck.test.js` couvre `deck.js` (jeu complet,
Fisher-Yates, distribution). Nécessite `npm install` au préalable.
```bash
cd web
npm install   # installe vitest (devDependency)
npm test
```

## Sécurité — origines autorisées
Le serveur restreint l'en-tête `Origin` du WebSocket (anti-CSWSH). En dev, les
origines Vite locales sont acceptées. En production, définissez la variable
d'environnement `ALLOWED_ORIGINS` (origines séparées par des virgules).
Remplacez le domaine ci-dessous par le vôtre — ce dépôt n'en publie aucun :
```bash
ALLOWED_ORIGINS="https://votre-domaine.exemple" go run .
```

### Important — Sprite SVG
Remplacez `web/public/cards.svg` par le fichier **Svg-cards-2.0.svg** renommé en `cards.svg`.  
Le code s’attend à des IDs du style `spade_king`, `heart_1`, etc., et `back` pour le dos.  
Le placeholder fourni ne contient que quelques symboles — il sert juste à voir l’appli démarrer.

## Licence
Ce projet est distribué sous **GNU Lesser General Public License v3.0**
(voir le fichier `LICENSE`).

L'asset `web/public/cards.svg` est **SVG-cards 2.0.1** © 2005 David Bellot,
distribué sous **GNU LGPL** (voir l'en-tête du fichier). Il reste régi par sa
propre licence quelle que soit la licence du code du projet.

**Ce fichier a été modifié** par rapport à l'original, le 2026-08-12 : ses 324
attributs `style="fill:…"` ont été convertis en attributs de présentation
`fill="…"`, sans autre changement. Le rendu est identique ; la raison est une
contrainte de sécurité expliquée ci-dessous. La LGPL autorise cette
modification, à condition qu'elle soit signalée — c'est l'objet de ce
paragraphe.

## Contrainte de sécurité — pas de style en ligne dans le sprite

Le sprite est injecté dans la page par `innerHTML` (`web/src/lib/svg-sprite.js`).
Ses attributs `style=` deviennent donc des **styles en ligne**, que refuse toute
CSP dépourvue de `style-src 'unsafe-inline'`. Le symptôme observé en production
était des cartes entièrement noires à bordure blanche, **sous Firefox
uniquement** : Gecko applique la règle là où Blink la laisse passer, et
l'absence de remplissage fait retomber les formes sur le noir par défaut.

Un attribut de présentation — `fill="#e6180a"` — n'est pas un style en ligne et
échappe entièrement à la CSP. C'est pourquoi le sprite n'en contient plus aucun.

**Si vous remplacez `cards.svg` par une autre version, convertissez ses
attributs `style=` avant de la mettre en service**, sinon la panne revient, et
elle revient en silence : les deux `catch` de `svg-sprite.js` n'écrivent rien
tant que `VERBOSE` vaut `false`.

## À faire côté jeu
- Logique d’état autoritaire côté serveur (distribution, tours, règles).
- Messages structurés (join/leave/action/state) — le squelette de convention est
  dans `web/src/lib/ws-client.js`.
- Anti-triche et persistance.
- Gestion des rooms multiples et reconnexion.
