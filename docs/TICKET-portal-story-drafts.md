# Ticket — Prévisualiser les US non publiées dans le portail

## Problème

Le portail annonce « Aucune US publiée » même à un administrateur qui possède
déjà des US non publiées dans son projet.

## Périmètre et solution

Afficher les US non publiées aux administrateurs du projet, avec la mention
« Brouillon » et la commande de publication. Les autres membres continuent de
voir uniquement les US publiées. Une US brouillon ne possède pas encore de
conversation client ouverte dans le portail.

## Critères d’acceptation

- Un administrateur retrouve ses US publiées et non publiées dans son projet.
- Un membre en lecture ne peut ni lister ni ouvrir une US non publiée.
- Les brouillons sont clairement distingués des US partagées avec les clients.
- Les tâches techniques et les commentaires internes ne sont pas exposés.

Scénarios : `features/client_stories.feature`.
