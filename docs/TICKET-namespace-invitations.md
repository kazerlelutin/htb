# Ticket — Invitations de namespace

## Problème

Inviter une personne à chaque projet d’un namespace est répétitif et oublie les
projets créés ultérieurement.

## Périmètre

Une invitation peut cibler exactement un projet ou un namespace. Son acceptation
crée une appartenance au namespace, appliquée dynamiquement à tous ses projets
actuels et futurs. Seul le propriétaire du namespace (ou un superadministrateur)
peut créer, lister ou révoquer ces invitations. Les invitations gardent leur
durée de vie de sept jours par défaut et leurs codes ne sont jamais renvoyés par
les listes.

## Critères d’acceptation

- `htb invite create --namespace MO5 --role read` crée un code à usage unique.
- Après acceptation, la personne peut voir et lire les projets existants et les
  projets ajoutés au namespace `MO5`.
- Les rôles `read`, `write` et `admin` sont appliqués comme pour les invitations
  de projet, avec la limite de rôle de l’identifiant déjà en vigueur.
- `htb invite list --namespace MO5` et `htb invite revoke ID --namespace MO5`
  permettent l’administration sans divulguer le code.
- Une invitation de namespace ne donne aucun droit sur un projet sans ce
  namespace, ni à une personne qui n’en est pas propriétaire pour l’administrer.
