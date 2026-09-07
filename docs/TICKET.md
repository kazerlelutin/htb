# HTB MVP

## Objectif

Livrer un gestionnaire de tickets terminal-first pour l'association : projets,
droits applicatifs, tickets hiérarchiques (US et tâches), bugs, incidents,
features produit, audit, versions et invitations Zitadel.

## Critères d'acceptation

- Une tâche technique appartient à une US du même projet et le statut de l'US
  est calculé depuis ses enfants.
- Bugs et incidents sont des tickets racines pouvant référencer une US ou une
  feature.
- Les membres voient les tickets autorisés par leur projet, y compris l'arbre.
- Les opérations sont auditables et les modifications de ticket révisables.
- Seul le claim Zitadel `superadmin` confère des droits globaux.
- Le service se déploie sans état sur CapRover avec PostgreSQL externe.
