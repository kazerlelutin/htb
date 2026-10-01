# CLI Windows / PowerShell

## Problème

Les releases produisent déjà `htb.exe`, mais aucun parcours d'installation
PowerShell vérifié ne permet de l'installer ni de le rendre disponible dans le
`PATH` utilisateur.

## Périmètre

- Installer la CLI Windows x86_64 depuis une release dans PowerShell.
- Vérifier SHA-256 avant toute installation.
- Rendre `htb` disponible dans la session en cours et les futures sessions.
- Vérifier que la CLI compile pour Windows dans l'intégration continue.

## Critères d'acceptation

- L'archive `htb_windows_amd64.zip` est installable sans privilège
  administrateur.
- Une somme de contrôle absente ou invalide empêche l'installation.
- `htb version` fonctionne après l'installation dans PowerShell.
- Le build Windows est contrôlé par la CI.
