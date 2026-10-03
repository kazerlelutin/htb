# Ticket — Afficher le compte connecté sur le site public

## Problème

Le lien « Se connecter / S’inscrire » reste présent après la connexion.

## Périmètre et solution

Résoudre la session navigateur sur les pages publiques et afficher le nom du
compte, une initiale en guise d’avatar et un bouton de déconnexion. Si la
session est absente ou expirée, conserver le lien de connexion. Le contenu
personnalisé ne doit pas être mis en cache.
Quand seul un identifiant technique est disponible, afficher « Mon tableau de
bord » à la place du nom et de l’avatar.
Lorsqu’un jeton fournit un username ou un e-mail, le préférer à un claim `name`
qui peut contenir l’identifiant technique Zitadel. Synchroniser cette identité
lors d’une authentification ultérieure afin de corriger les comptes existants.
Placer les actions de compte à droite, à côté des langues, avec une disposition
qui reste utilisable sur mobile.

## Critères d’acceptation

- Le nom et l’initiale du compte connecté sont échappés et mènent au portail.
- Un identifiant technique ne figure jamais dans le lien vers le portail.
- Le username est affiché en priorité, puis l’e-mail ; aucun ne laisse place à
  l’identifiant technique Zitadel.
- La déconnexion reste possible depuis les pages publiques.
- Une session expirée ne révèle aucune information du compte.
- Les libellés français et anglais restent adaptés à la langue de la page.
- Le compte et les langues forment un groupe à droite ; sur mobile, les liens
  restent visibles et utilisables sans débordement horizontal.

Scénarios : `features/browser_auth.feature`.
