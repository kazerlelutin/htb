# Ticket — MCP HTB pour lire et proposer dans ChatGPT

## Problème

Les personnes qui utilisent HTB ne peuvent pas consulter leurs projets et
tickets depuis ChatGPT. L'API HTTP existe, mais elle n'expose pas le protocole
MCP ni la découverte OAuth attendue par ChatGPT.

## Solution

Exposer un endpoint MCP Streamable HTTP à `/mcp`, protégé par les jetons
Zitadel déjà validés par HTB. Il permet de lire les projets et tickets autorisés
pour la personne connectée, ainsi que de soumettre une proposition au processus
de demande existant après confirmation explicite. Il expose également la
ressource OAuth protégée afin que ChatGPT puisse démarrer le flux OAuth
Authorization Code + PKCE configuré dans Zitadel.

## Critères d’acceptation

- `POST /mcp` répond à `initialize`, `tools/list` et `tools/call`.
- Le serveur répond à `server/discover` du protocole MCP moderne (`2026-07-28`)
  afin que ChatGPT puisse découvrir les actions avant de les appeler.
- Les outils disponibles permettent la liste des projets, la liste des tickets
  d’un projet, la lecture d’un ticket et la soumission confirmée d’une
  proposition dans les demandes client existantes.
- Une proposition MCP est autorisée pour un membre `read` du projet, requiert
  une confirmation explicite et ne crée ni ne modifie directement un ticket.
- La découverte des outils décrit OAuth par `securitySchemes` et par son miroir
  de compatibilité `_meta.securitySchemes`.
- Les instructions MCP indiquent clairement que le chat planifie et propose ;
  les agents HTB exécutent après le processus de demande et de tri existant.
- Chaque appel MCP vérifie le jeton Zitadel et passe par les autorisations HTB
  existantes ; aucun jeton de CLI n'est demandé ni stocké.
- Une requête non authentifiée reçoit un challenge OAuth et la métadonnée
  `/.well-known/oauth-protected-resource` décrit la ressource MCP.
- Les guides d’auto-hébergement expliquent la configuration et la connexion
  dans ChatGPT, y compris que l’URI de redirection OAuth est enregistrée
  dynamiquement par ChatGPT et non saisie dans une application Zitadel.

Scénario : `features/mcp_chatgpt.feature`.
