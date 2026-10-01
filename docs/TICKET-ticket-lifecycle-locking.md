# Verrouillage du cycle de vie des tickets

## Problème

L'archivage d'un ticket utilise une jointure externe pour retrouver son parent.
PostgreSQL refuse `FOR UPDATE` sur le côté nullable de cette jointure, ce qui
empêche notamment le contrôle d'autorisation d'un lecteur.

## Périmètre

Verrouiller uniquement la ligne du ticket lors de son archivage ou de sa
restauration.

## Critères d'acceptation

- Un lecteur ne peut toujours pas archiver le ticket d'un autre utilisateur.
- Le créateur et un administrateur peuvent archiver ou restaurer le ticket.
- Le test d'intégration du cycle de vie passe sur PostgreSQL.
