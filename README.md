# Table de cartes en ligne — serveur Go autoritaire + front Svelte

Une **table de cartes virtuelle** pour un groupe fermé de joueurs. Le principe
tient en une phrase : le logiciel reproduit une table, des cartes et un sabot,
et **ne connaît aucune règle de jeu**. On distribue, on retourne, on empile, on
donne une carte à quelqu'un — exactement comme autour d'une vraie table. À
chaque joueur de savoir ce qu'il joue.

Ce que le serveur sait faire, et ce qu'il ignore délibérément :

| Il sait | Il ignore |
|---|---|
| où est chaque carte (sabot / tapis / main) | ce que vaut une carte |
| qui la possède, si elle est face visible | de qui c'est le tour |
| son ordre de superposition, sa rotation | ce qui constitue un coup légal |
| qui est connecté, et combien de cartes il a en main | qui gagne |

> **L'état est autoritaire côté serveur.** Le client n'est qu'un miroir : il
> demande une mutation, le serveur l'applique sous verrou et rediffuse l'état.
> Aucune carte n'est déplacée « localement d'abord ».

## Architecture

    navigateur                      serveur Go (:8080)
    ┌──────────────────┐            ┌────────────────────────┐
    │ Svelte + GSAP    │  POST      │ /api/login  → token    │
    │  Login           │ ─────────► │   bcrypt + limiteur    │
    │  InitMenu        │            ├────────────────────────┤
    │  Table / Hand    │  WS ?token │ /ws → hub (rooms)      │
    │  Chat            │ ◄────────► │   dispatch autoritaire │
    │  store.js        │  état      │   engine (mutex)       │
    └──────────────────┘            └────────────────────────┘

Le front et le serveur partagent **une seule origine** : en développement Vite
relaie `/api` et `/ws` vers `:8080`, en production nginx (ou Caddy) fait la même
chose. Le client WebSocket se connecte donc toujours à l'origine de la page.

Deux flux de diffusion coexistent, et c'est la clé du modèle :

- **l'état public** (`state`) — sabot en nombre, cartes du tapis, joueurs
  connectés et **compte** de leurs cartes — est rediffusé à toute la salle après
  chaque mutation ;
- **la main privée** (`hand`) n'est envoyée qu'à son propriétaire. Elle
  n'apparaît jamais dans l'état public : c'est pourquoi retourner une carte de
  sa main déclenche un envoi ciblé en plus de la diffusion générale.

## Arborescence

```
cardgame-svelte-gsap-go/
├─ server/                 # Go 1.25, dépendances : gorilla/websocket, x/crypto
│  ├─ main.go              # upgrade WS, heartbeat, cycle de vie d'un client
│  ├─ auth.go              # comptes, bcrypt, sessions, limiteur de connexion
│  ├─ handlers.go          # dispatch des messages, diffusion état/main
│  ├─ state.go             # engine : l'état autoritaire et ses mutations
│  ├─ deck.go              # composition du sabot d'après la config du menu
│  ├─ hub.go               # salles, inscription/retrait, broadcast
│  ├─ *_test.go            # deck, hub, state  (auth et handlers : voir Dette)
│  ├─ users.txt.example    # gabarit de comptes — vide, à copier
│  └─ Dockerfile
└─ web/                    # Svelte 4 + Vite 5 + GSAP
   ├─ public/cards.svg     # sprite SVG-cards 2.0.1 (voir Licence)
   ├─ src/
   │  ├─ App.svelte        # étapes auth → connexion → init | table
   │  └─ lib/
   │     ├─ Login.svelte      InitMenu.svelte   # entrée dans la partie
   │     ├─ Table.svelte      # tapis, sélection multiple, hit-test des drops
   │     ├─ Card.svelte       Hand.svelte  Sabot.svelte  Avatar.svelte
   │     ├─ Chat.svelte
   │     ├─ store.js          # état client (miroir) + session
   │     ├─ ws-client.js      # client WS : ping/pong, reconnexion, envois typés
   │     ├─ svg-sprite.js     # injection du sprite, cadrage des symboles
   │     ├─ drag.js  cards-anim.js
   │     └─ deck.js           # hérité du squelette — voir Dette
   ├─ nginx.conf           # sert le build + relaie /api et /ws
   └─ Dockerfile
```

## Prérequis

- **Go 1.25+**
- **Node 18+**

## Faire tourner le jeu en local

Il faut **deux terminaux**. Le serveur Go d'abord :

```powershell
cd server
$env:ALLOW_DEMO_USERS="1"        # PowerShell ; sh : export ALLOW_DEMO_USERS=1
go run .
```

Sans `ALLOW_DEMO_USERS` — et sans `USERS_FILE` ni `USERS_SEED` — **le serveur
refuse de démarrer** sur `Aucun compte défini`. Ce n'est pas une panne : c'est
un refus délibéré de tourner sans comptes (`auth.go`, `bootstrapUsers`). Avec la
variable, deux comptes de démonstration apparaissent : `alice/secret` et
`bob/secret`.

Le front ensuite :

```bash
cd web
npm install     # la première fois seulement
npm run dev
```

Puis ouvrir <http://localhost:5173>.

**Jouer à deux sans deuxième machine** : ouvrir une fenêtre de navigation privée,
ou un autre navigateur, et se connecter comme `bob`. La session est gardée par
onglet — deux onglets ordinaires du même navigateur partagent le même compte.

**Tester le front tel qu'il sera déployé** (code minifié, pas de rechargement à
chaud) : `npm run build && npm run preview`. Le port 4173 figure déjà dans la
liste blanche d'origines, le WebSocket est donc accepté sans réglage.

Vérifications rapides, serveur démarré :

| Commande | Attendu |
|---|---|
| `curl http://localhost:8080/api/health` | `204` |
| `curl -X POST http://localhost:8080/api/login -H "Content-Type: application/json" -d "{\"username\":\"alice\",\"password\":\"secret\"}"` | `200` + jeton |
| la même, mauvais mot de passe | `401` |
| `curl http://localhost:5173/api/health` | `204` — prouve que le relais Vite fonctionne |

> Attention au limiteur : **cinq tentatives de connexion par minute**. Quelques
> essais ratés de suite renvoient `429` pendant une minute. C'est voulu.

## Comptes

Les comptes sont **préenregistrés au démarrage**, jamais créés depuis
l'interface : le jeu est destiné à un cercle fermé. Les mots de passe sont
hachés en bcrypt en mémoire ; rien n'est persisté, un redémarrage invalide tous
les jetons de session.

Trois sources, par ordre de priorité :

| Variable | Format | Usage |
|---|---|---|
| `USERS_FILE` | chemin d'un fichier, une ligne `nom:motdepasse` | **production** |
| `USERS_SEED` | `nom:motdepasse,nom2:motdepasse2` | pratique en conteneur |
| `ALLOW_DEMO_USERS=1` | — | crée `alice`/`bob`, **tests locaux uniquement** |

Copier `server/users.txt.example` en `server/users.txt` et le remplir. Ce
fichier est exclu du suivi git et doit l'être : `chmod 600` sur un serveur.
Lignes vides et lignes commençant par `#` sont ignorées ; une ligne sans mot de
passe est ignorée **en silence**, et le compte correspondant n'existe alors
tout simplement pas.

## Protocole WebSocket

Le client s'authentifie par `?token=` sur l'upgrade — sans jeton valide, `401`
et pas d'upgrade. La convention complète est documentée en tête de
`web/src/lib/ws-client.js`, qui expose un envoi typé par message. En résumé :

| Client → serveur | Effet |
|---|---|
| `init` | construit le sabot d'après la config du menu (optionnellement mélangé) |
| `flip` `front` `rotate` `move` | mutations sur une carte |
| `flipMany` `moveMany` `transferMany` | mêmes opérations par lot (sélection multiple), **une** diffusion |
| `transfer` `sabotDraw` | changement de zone ; `sabotDraw` tire le sommet du sabot |
| `drag` `dragMany` `dragEnd` | positions live pendant un glisser, simple relais aux autres |
| `chat` | relayé **signé** : le serveur réinjecte le nom authentifié |
| `ping` | keep-alive applicatif |

| Serveur → client | Portée |
|---|---|
| `state` | toute la salle : sabot (nombre), tapis, joueurs, `initialized` |
| `hand` | le propriétaire seul |
| `drag` `dragMany` `dragEnd` | relais des glissers des autres |
| `chat` `pong` | — |

Les messages `drag*` sont le seul flux qui **ne passe pas** par l'état
autoritaire : ce sont des positions éphémères, destinées à la seule fluidité
visuelle. Un `dragEnd` demande aux autres clients d'effacer ces positions et de
revenir à l'état serveur — sans lui, un glisser annulé laisserait des cartes
fantômes chez les spectateurs.

Le serveur valide la **cohérence technique** (carte existante, zone compatible,
lot borné à 512 cartes, config produisant au moins une carte), jamais la
légalité d'un coup — il n'y a pas de coup légal.

## Ce que sait faire la table

- **Composition du sabot** au menu d'initialisation : nombre de jeux (1 à 8),
  couleurs et rangs à la carte, jokers (aucun / noir / rouge / les deux),
  mélange Fisher-Yates optionnel.
- **Glisser-déposer** entre sabot, tapis, main et avatars des autres joueurs.
  Une carte donnée depuis le sabot est révélée à son destinataire ; un simple
  déplacement ne change **jamais** l'état face — un joueur qui a retourné sa
  carte avant de la poser voit son choix respecté.
- **Sélection multiple** : rectangle tracé sur le feutre, `Maj`+clic pour
  ajouter ou retirer, `Ctrl`+clic pour saisir une pile entière (reconstruite
  géométriquement — le serveur n'a pas de notion de pile). Puis empiler, étaler,
  retourner (`F`) ou transférer le lot en une seule mutation. `Échap` vide la
  sélection.
- **Aimantage** optionnel d'une carte lâchée près d'une autre.
- **Avatars** répartis autour de la table, portant le nombre de cartes en main.
- **Chat**, statut de connexion, reconnexion automatique avec backoff.
- **Nouvelle partie** sous confirmation : le sabot est remplacé et **toutes**
  les mains sont vidées, immédiatement, pour tout le monde.

Rejoindre une partie déjà en cours ouvre directement la table ; le menu
d'initialisation ne s'affiche que si aucun sabot n'est chargé.

## Tests

```bash
cd server && go vet ./... && go test ./...
cd web    && npm test
```

Couvert : `deck.go` (composition, normalisation de config, mélange), `hub.go`
(join/leave, broadcast, isolation par salle, concurrence), `state.go` (mutations,
transferts, lots, snapshots) — 34 tests Go. Côté front, `deck.test.js` couvre
`deck.js`. Les manques sont listés plus bas.

## Sécurité

**Origines WebSocket.** Le serveur restreint l'en-tête `Origin` (anti-CSWSH).
Les origines Vite locales (`5173`, `4173`) sont acceptées d'office ; en
production, ajouter la vôtre :

```bash
ALLOWED_ORIGINS="https://votre-domaine.exemple" go run .
```

Ce dépôt ne publie aucun domaine réel.

**Deux limites assumées, à connaître avant de vouloir les « corriger »** :

- une requête **sans** en-tête `Origin` est acceptée (curl, clients non
  navigateur). Retirer cette branche dans `checkOrigin` pour exiger une origine
  explicite.
- le limiteur de connexion compte par `RemoteAddr`, donc **globalement** dès
  qu'un reverse proxy est en amont. C'est délibéré : `X-Forwarded-For` est
  falsifiable par le client, et s'y fier permettrait de contourner la limite à
  volonté. Tant que les mots de passe des joueurs sont courts, ce défaut est
  précisément ce qui rend une attaque par force brute longue.

L'énumération de comptes par chronométrage est écartée : un identifiant inconnu
déclenche quand même une comparaison bcrypt bidon.

## Déploiement

`docker compose up -d --build` construit les deux images et publie le front sur
le port 8090. Il faut au préalable `server/users.txt` et la variable
`ALLOWED_ORIGINS` (voir l'en-tête de `docker-compose.yml`). nginx sert le build
statique et relaie `/api` et `/ws` vers le conteneur Go, avec un
`proxy_read_timeout` de 120 s — le serveur envoie un ping toutes les 30 s.

Un déploiement sans conteneur (binaire Go + Caddy) est également en service ;
sa documentation vit hors de ce dépôt.

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

## Dette connue

Rien de ceci n'est cassé ; ce sont des manques assumés, listés pour qui reprend
le projet.

- **`auth.go` et `handlers.go` n'ont aucun test** — ce sont pourtant les
  fichiers qui portent les jetons, le limiteur et le dispatch.
- **Le front n'est testé que sur `deck.js`.** `Table.svelte` et `svg-sprite.js`,
  soit le code le plus retors, ne le sont pas.
- **`web/src/lib/deck.js` est hérité du squelette** : depuis que le serveur
  construit le sabot, plus personne ne l'importe. Seuls ses tests le
  maintiennent en vie. À supprimer, ou à réhabiliter s'il doit servir.
- **`VERBOSE` est codé en dur à `false`** dans `svg-sprite.js`, alors que le
  commentaire voisin promet l'inverse. À lier à `import.meta.env.DEV`.
- **Cinq dépendances ont pris du retard**, sans faille connue : Svelte 4 → 5
  (changement du modèle de réactivité), Vite 5 → 8, Vitest 1 → 4,
  `@sveltejs/vite-plugin-svelte` 3 → 7, GSAP 3.13 → 3.15. La montée est un
  chantier à part entière, pas une mise à jour de passage.
- **Aucune persistance** : comptes, sessions et partie en cours vivent en
  mémoire. Redémarrer le serveur vide la table. C'est un choix, pas un oubli —
  mais `UserStore` est une interface, précisément pour qu'un backend puisse s'y
  substituer sans toucher au reste.
- **Une seule salle** en pratique (`lobby`). Le hub en gère plusieurs, mais rien
  côté interface ne permet d'en choisir une autre.
