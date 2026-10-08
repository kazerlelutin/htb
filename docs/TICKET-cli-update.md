# Ticket — Mise à jour de la CLI

## Problème

Mettre à jour HTB demande de retrouver puis de rejouer l’installateur adapté à
son système. Le parcours est répétitif et ne couvre pas macOS par l’installateur
actuel, alors que les archives de release correspondantes existent.

## Solution

Ajouter `htb update`. La commande sélectionne l’archive publiée pour la
plateforme courante, vérifie sa somme SHA-256 depuis la même release, puis
remplace l’exécutable en cours. Sous Windows, où l’exécutable est verrouillé
pendant son exécution, le remplacement est lancé après la fin de la commande.

## Critères d’acceptation

- `htb update` prend en charge Linux x86_64, macOS x86_64/ARM64 et Windows
  x86_64, qui sont les cibles des releases.
- L’archive ne peut pas être installée si sa somme SHA-256 est absente ou
  différente de celle publiée.
- Une plateforme non distribuée reçoit une erreur explicite sans téléchargement.
- La commande est documentée, visible dans l’aide et couverte par les scénarios
  de la CLI.
