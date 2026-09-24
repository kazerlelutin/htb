# Ticket — Barres de progression du portail

## Problème

Les barres de progression sont trop fines et leur piste native paraît grise,
contrairement aux séparateurs du portail.

## Périmètre et solution

Augmenter la hauteur des barres du portail à 1,5 rem, rendre la piste
transparente et utiliser la couleur des séparateurs pour leur bordure. Utiliser
un vert sauge plus doux pour le remplissage, sans changer les libellés de
progression ni le vert d’accent des contrôles interactifs.

## Critères d’acceptation

- Les barres du projet et des US ont une hauteur de 1,5 rem.
- Leur partie non remplie laisse voir le fond de la page ou de la carte.
- Leur bordure utilise `--line`, comme les séparateurs du portail.
- Leur remplissage utilise `--progress` (`#9ab87a`), moins vif que `--lime`.
- Une barre à 0 % reste visible grâce à sa bordure.

Scénario : `features/client_stories.feature`.
