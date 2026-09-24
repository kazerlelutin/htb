# Ticket — Expiration par défaut des invitations

## Problème

La commande `htb invite create` présente `--expires-at` comme facultatif, mais
elle envoie une valeur vide si l’option est absente. L’API attendait alors une
date et rejetait la demande.

## Solution

Quand aucune date n’est fournie, la CLI n’envoie pas le champ et l’API attribue
une expiration de sept jours. Une date RFC 3339 explicite reste prioritaire.

## Critères d’acceptation

- `htb invite create --project ANBY --role read` crée une invitation valide.
- Une invitation sans date expire sept jours après sa création.
- `--expires-at` permet de choisir une date d’expiration différente.
- Une invitation expire toujours et les contrôles d’autorisation existants ne
  changent pas.

Scénario : `features/invitations.feature`.
