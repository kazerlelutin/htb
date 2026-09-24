package httpapi

import (
	"html/template"
	"strings"
)

const hostedHTBURL = "https://htb.ben-to.fr"

// deploymentModesBody links to the hosted instance and to the GitHub
// self-hosting guide. A local /login link appears only when this instance
// actually has browser sign-in configured.
func deploymentModesBody(language string, browserLogin bool, publicURL string) template.HTML {
	selfHostGuide := "https://github.com/kazerlelutin/htb/blob/main/docs/self-hosting.md"
	if language == "fr" {
		selfHostGuide = "https://github.com/kazerlelutin/htb/blob/main/docs/self-hosting.fr.md"
	}
	hostedLink := hostedHTBURL
	hostedLabel := localized(language, "Ouvrir la version en ligne", "Open the hosted service")
	if strings.TrimRight(publicURL, "/") == hostedHTBURL {
		if browserLogin {
			hostedLink = "/login"
			hostedLabel = localized(language, "Accéder au portail client", "Open the client portal")
		} else {
			hostedLink = "/downloads?lang=" + language
			hostedLabel = localized(language, "Installer la CLI", "Install the CLI")
		}
	}
	if language == "en" {
		return template.HTML(`<section class="deployment-options" id="deployment" aria-labelledby="deployment-title"><p class="eyebrow">YOUR CHOICE</p><h2 id="deployment-title">Use our server or run your own.</h2><div class="deployment-grid"><article><h3>Hosted by Ben-to</h3><p>Connect to htb.ben-to.fr. We run the service; you use the CLI and invite clients to the portal.</p><code>htb config set-server https://htb.ben-to.fr</code><a class="button" href="` + hostedLink + `">` + hostedLabel + ` <span aria-hidden="true">→</span></a></article><article><h3>Self-host HTB</h3><p>Run HTB on your server and keep control of its data. The setup guide covers the database, sign-in, and HTTPS.</p><a class="button secondary" href="` + selfHostGuide + `">Read the setup guide <span aria-hidden="true">→</span></a></article></div></section>`)
	}
	return template.HTML(`<section class="deployment-options" id="deployment" aria-labelledby="deployment-title"><p class="eyebrow">À VOUS DE CHOISIR</p><h2 id="deployment-title">Utilisez notre serveur ou hébergez le vôtre.</h2><div class="deployment-grid"><article><h3>Version hébergée par Ben-to</h3><p>Connectez-vous à htb.ben-to.fr. Nous exploitons le service ; vous utilisez la CLI et invitez vos clients sur le portail.</p><code>htb config set-server https://htb.ben-to.fr</code><a class="button" href="` + hostedLink + `">` + hostedLabel + ` <span aria-hidden="true">→</span></a></article><article><h3>Auto-héberger HTB</h3><p>Déployez HTB sur votre serveur et gardez la main sur ses données. Le guide couvre la base, la connexion et HTTPS.</p><a class="button secondary" href="` + selfHostGuide + `">Lire le guide d’installation <span aria-hidden="true">→</span></a></article></div></section>`)
}
