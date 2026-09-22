package httpapi

import (
	_ "embed"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"strings"
)

//go:embed assets/public.css
var publicCSS string

//go:embed assets/public.js
var publicJS string

//go:embed assets/favicon.svg
var publicFavicon string

type publicPage struct {
	Language, Title, Description, Canonical, FrenchURL, EnglishURL string
	Body                                                           template.HTML
}

var publicLayout = template.Must(template.New("public").Parse(`<!doctype html>
<html lang="{{.Language}}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="description" content="{{.Description}}"><link rel="canonical" href="{{.Canonical}}"><meta name="theme-color" content="#101414">
<link rel="icon" type="image/svg+xml" href="/favicon.svg"><link rel="stylesheet" href="/assets/public.css"><title>{{.Title}}</title></head><body>
<a class="skip-link" href="#main">{{if eq .Language "fr"}}Aller au contenu{{else}}Skip to content{{end}}</a>
<header class="site-header"><a class="wordmark" href="/?lang={{.Language}}" aria-label="HTB home"><span aria-hidden="true">&gt;</span><span class="cursor" aria-hidden="true">_</span> HTB</a><nav aria-label="{{if eq .Language "fr"}}Navigation principale{{else}}Main navigation{{end}}"><a href="/downloads?lang={{.Language}}">{{if eq .Language "fr"}}Télécharger{{else}}Download{{end}}</a><a href="/cgu?lang={{.Language}}">{{if eq .Language "fr"}}CGU{{else}}Terms{{end}}</a></nav><nav class="languages" aria-label="Language"><a href="{{.FrenchURL}}" lang="fr" hreflang="fr">FR</a><span aria-hidden="true">/</span><a href="{{.EnglishURL}}" lang="en" hreflang="en">EN</a></nav></header>
<main id="main">{{.Body}}</main>
<footer class="site-footer"><span>HTB — Headless Ticket Board</span><nav aria-label="{{if eq .Language "fr"}}Informations légales{{else}}Legal information{{end}}"><a href="/mentions-legales?lang={{.Language}}">{{if eq .Language "fr"}}Mentions légales{{else}}Legal notice{{end}}</a><a href="/privacy?lang={{.Language}}">{{if eq .Language "fr"}}Confidentialité{{else}}Privacy{{end}}</a><button class="link-button" type="button" data-open-consent>{{if eq .Language "fr"}}Préférences de mesure{{else}}Analytics preferences{{end}}</button></nav></footer>
<section class="consent" id="consent" aria-label="{{if eq .Language "fr"}}Préférences de mesure{{else}}Analytics preferences{{end}}" role="dialog" aria-modal="false" hidden><div><h2>{{if eq .Language "fr"}}Votre vie privée{{else}}Your privacy{{end}}</h2><p>{{if eq .Language "fr"}}Avec votre accord, HTB utilise une mesure d’audience hébergée par Ben-to pour améliorer le site. Vous pouvez refuser sans conséquence.{{else}}With your permission, HTB uses Ben-to-hosted analytics to improve this site. You can refuse without any consequence.{{end}}</p><p><a href="/privacy?lang={{.Language}}">{{if eq .Language "fr"}}En savoir plus{{else}}Learn more{{end}}</a></p></div><div class="consent-actions"><button class="button secondary" type="button" data-consent="rejected">{{if eq .Language "fr"}}Refuser{{else}}Reject{{end}}</button><button class="button" type="button" data-consent="accepted">{{if eq .Language "fr"}}Accepter{{else}}Accept{{end}}</button></div></section>
<script defer src="/assets/public.js"></script></body></html>`))

func publicLanguage(r *http.Request) string {
	if r.URL.Query().Get("lang") == "en" || (r.URL.Query().Get("lang") == "" && strings.HasPrefix(strings.ToLower(r.Header.Get("Accept-Language")), "en")) {
		return "en"
	}
	return "fr"
}

func languageURL(r *http.Request, language string) string { return r.URL.Path + "?lang=" + language }

func localized(language, french, english string) string {
	if language == "en" {
		return english
	}
	return french
}

func (s *Server) renderPublicPage(w http.ResponseWriter, r *http.Request, title, description string, body template.HTML) {
	language := publicLanguage(r)
	s.publicHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	page := publicPage{language, title, description, s.publicURL + r.URL.Path, languageURL(r, "fr"), languageURL(r, "en"), body}
	if err := publicLayout.Execute(w, page); err != nil {
		s.log.Error("render public page", "error", err)
	}
}

func (s *Server) publicHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; form-action 'self'; script-src 'self' https://analytics.ben-to.fr; style-src 'self'; img-src 'self' data: https://analytics.ben-to.fr; connect-src 'self' https://analytics.ben-to.fr; font-src 'self'; upgrade-insecure-requests")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}

func (s *Server) publicStyles(w http.ResponseWriter, _ *http.Request) {
	s.publicHeaders(w)
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	fmt.Fprint(w, publicCSS)
}

func (s *Server) publicScript(w http.ResponseWriter, _ *http.Request) {
	s.publicHeaders(w)
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	fmt.Fprint(w, publicJS)
}

func (s *Server) favicon(w http.ResponseWriter, _ *http.Request) {
	s.publicHeaders(w)
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	fmt.Fprint(w, publicFavicon)
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	language := publicLanguage(r)
	body := homeBody(language) + simulatorBody(language)
	s.renderPublicPage(w, r, localized(language, "HTB — Tickets pour le terminal", "HTB — Tickets for the terminal"), localized(language, "HTB est un gestionnaire de tickets pensé pour le terminal, les scripts et les agents.", "HTB is a ticket tracker designed for terminals, scripts, and agents."), body)
}

func (s *Server) legalNotice(w http.ResponseWriter, r *http.Request) {
	language := publicLanguage(r)
	s.renderPublicPage(w, r, localized(language, "Mentions légales — HTB", "Legal notice — HTB"), localized(language, "Informations légales du service HTB.", "Legal information for HTB."), legalNoticeBody(language))
}

func (s *Server) terms(w http.ResponseWriter, r *http.Request) {
	language := publicLanguage(r)
	s.renderPublicPage(w, r, localized(language, "CGU — HTB", "Terms of service — HTB"), localized(language, "Conditions générales d’utilisation de HTB.", "HTB terms of service."), termsBody(language))
}

func (s *Server) privacy(w http.ResponseWriter, r *http.Request) {
	language := publicLanguage(r)
	s.renderPublicPage(w, r, localized(language, "Confidentialité et cookies — HTB", "Privacy and cookies — HTB"), localized(language, "Politique de confidentialité et de cookies de HTB.", "HTB privacy and cookies policy."), privacyBody(language))
}

func homeBody(language string) template.HTML {
	if language == "en" {
		return template.HTML(`<section class="hero"><p class="eyebrow">TERMINAL-FIRST ISSUE TRACKING</p><h1>Keep work moving,<br><em>without leaving your terminal.</em></h1><p class="lede">HTB gives people, scripts, and agents one durable ticket board. Create, query, automate, and keep the history.</p><div class="actions"><a class="button" href="/downloads">Download HTB <span aria-hidden="true">→</span></a><a class="button secondary" href="#workflow">See a workflow</a></div><div class="terminal" aria-label="HTB command example"><div class="terminal-bar"><span></span><span></span><span></span><code>~/work</code></div><pre><code><b>$</b> htb project create --key SITE --name "Website"
<span class="success">Project SITE created. It is now current.</span>
<b>$</b> htb ticket create --title "Ship the new homepage"
<span class="muted">Created SITE-1 · user_story · open</span></code></pre></div></section><section class="features"><article><span>01</span><h2>Built for commands</h2><p>Your workflow stays keyboard-first. The CLI is readable for humans and offers JSON and CSV for automation.</p></article><article><span>02</span><h2>Useful structure</h2><p>Stories, technical tasks, bugs, incidents, roadmap features, comments, and revision history live in one small model.</p></article><article><span>03</span><h2>Shared without the noise</h2><p>Invite the right people. Keep project permissions explicit and let every change remain traceable.</p></article></section><section class="workflow" id="workflow"><div><p class="eyebrow">A CONCRETE WORKFLOW</p><h2>Turn a delivery into<br>a sequence your team can trust.</h2><p>Start a project, describe the user story, split its technical work, then let the story progress follow its tasks.</p></div><ol><li><code>htb project create --key API --name "Public API"</code></li><li><code>htb ticket create --type user_story --title "Let clients export data"</code></li><li><code>htb ticket create --type technical_task --parent API-1 --title "Add CSV endpoint"</code></li></ol></section><section class="difference"><p class="eyebrow">NOT ANOTHER KANBAN</p><h2>HTB is the ticket layer your terminal can speak.</h2><p>Traditional ticket tools begin with boards and browser workflows. HTB begins with an API and a CLI: quicker for focused work, scriptable for recurring work, and legible to coding agents.</p><a class="text-link" href="/downloads">Read the complete command guide <span aria-hidden="true">→</span></a></section>`)
	}
	return template.HTML(`<section class="hero"><p class="eyebrow">SUIVI DE TICKETS TERMINAL-FIRST</p><h1>Faites avancer le travail,<br><em>sans quitter le terminal.</em></h1><p class="lede">HTB donne aux personnes, scripts et agents un même tableau de tickets durable. Créez, interrogez, automatisez et gardez l’historique.</p><div class="actions"><a class="button" href="/downloads">Télécharger HTB <span aria-hidden="true">→</span></a><a class="button secondary" href="#workflow">Voir un cas d’usage</a></div><div class="terminal" aria-label="Exemple de commandes HTB"><div class="terminal-bar"><span></span><span></span><span></span><code>~/projet</code></div><pre><code><b>$</b> htb project create --key SITE --name "Site web"
<span class="success">Projet SITE créé. Il est maintenant courant.</span>
<b>$</b> htb ticket create --title "Livrer la nouvelle page d’accueil"
<span class="muted">SITE-1 créé · user_story · open</span></code></pre></div></section><section class="features"><article><span>01</span><h2>Conçu pour la commande</h2><p>Le flux reste au clavier. La CLI est lisible pour les humains et fournit du JSON et CSV pour l’automatisation.</p></article><article><span>02</span><h2>Une structure utile</h2><p>US, tâches techniques, bugs, incidents, fonctionnalités de roadmap, commentaires et historique vivent dans un modèle concis.</p></article><article><span>03</span><h2>Partagé sans bruit</h2><p>Invitez les bonnes personnes. Les droits de projet sont explicites et chaque changement reste traçable.</p></article></section><section class="workflow" id="workflow"><div><p class="eyebrow">UN CAS D’USAGE CONCRET</p><h2>Transformez une livraison<br>en séquence fiable.</h2><p>Créez un projet, décrivez l’US, découpez le travail technique, puis laissez la progression de l’US suivre ses tâches.</p></div><ol><li><code>htb project create --key API --name "API publique"</code></li><li><code>htb ticket create --type user_story --title "Permettre l’export"</code></li><li><code>htb ticket create --type technical_task --parent API-1 --title "Ajouter le CSV"</code></li></ol></section><section class="difference"><p class="eyebrow">PAS UN KANBAN DE PLUS</p><h2>HTB est la couche ticket que votre terminal sait parler.</h2><p>Les outils de tickets traditionnels commencent par les tableaux et le navigateur. HTB commence par une API et une CLI : plus rapide pour le travail concentré, scriptable pour le répétitif, et lisible par les agents de code.</p><a class="text-link" href="/downloads">Lire le guide complet des commandes <span aria-hidden="true">→</span></a></section>`)
}

func simulatorBody(language string) template.HTML {
	if language == "en" {
		return template.HTML(`<section class="simulator" id="simulator" aria-labelledby="simulator-title"><div class="simulator-copy"><p class="eyebrow">INTERACTIVE PREVIEW</p><h2 id="simulator-title">See a workday,<br>not just commands.</h2><p>Explore four project moments. This local simulation does not create an account, contact the API, or save anything.</p><div class="simulator-tabs" role="tablist" aria-label="HTB workflow"><button type="button" role="tab" id="simulator-tab-create" aria-selected="true" aria-controls="simulator-create" data-simulator-tab="simulator-create">Create</button><button type="button" role="tab" id="simulator-tab-track" aria-selected="false" aria-controls="simulator-track" tabindex="-1" data-simulator-tab="simulator-track">Track</button><button type="button" role="tab" id="simulator-tab-share" aria-selected="false" aria-controls="simulator-share" tabindex="-1" data-simulator-tab="simulator-share">Share</button><button type="button" role="tab" id="simulator-tab-automate" aria-selected="false" aria-controls="simulator-automate" tabindex="-1" data-simulator-tab="simulator-automate">Automate</button></div></div><div class="simulator-output"><section class="terminal simulator-panel" id="simulator-create" role="tabpanel" aria-labelledby="simulator-tab-create"><div class="terminal-bar"><span></span><span></span><span></span><code>~/website</code></div><pre><code><b>$</b> htb ticket create --title "Publish pricing"
<span class="success">Ticket SITE-12 created — open.</span>

<b>$</b> htb ticket create --type technical_task \
  --parent SITE-12 --title "Add comparison table"
<span class="muted">Ticket SITE-13 created — open.</span></code></pre><p class="simulator-caption">A story and its technical work stay connected from the first command.</p></section><section class="terminal simulator-panel" id="simulator-track" role="tabpanel" aria-labelledby="simulator-tab-track" hidden><div class="terminal-bar"><span></span><span></span><span></span><code>~/website</code></div><pre><code><b>$</b> htb project status
<span class="success">* SITE — Website</span>
  User stories  <span class="success">[██████░░░░]</span> 3/5 (60%)
  Tickets       <span class="success">[███████░░░]</span> 7/9 (77%)
  open 1 · in_progress 1 · review 0 · blocked 0 · done 7</code></pre><p class="simulator-caption">See delivery progress without building a manual report.</p></section><section class="terminal simulator-panel" id="simulator-share" role="tabpanel" aria-labelledby="simulator-tab-share" hidden><div class="terminal-bar"><span></span><span></span><span></span><code>~/website</code></div><pre><code><b>$</b> htb invite create --role write
<span class="success">Invitation code created: bramble-forest</span>
Share it with: htb invite accept bramble-forest

<b>$</b> htb ticket claim SITE-13
<span class="muted">Ticket SITE-13 claimed.</span></code></pre><p class="simulator-caption">Give the right access, then make ownership visible.</p></section><section class="terminal simulator-panel" id="simulator-automate" role="tabpanel" aria-labelledby="simulator-tab-automate" hidden><div class="terminal-bar"><span></span><span></span><span></span><code>~/website</code></div><pre><code><b>$</b> htb ticket list --json
[
  {"ref":"SITE-12","status":"in_progress"}
]

<b>$</b> htb ticket comment SITE-12 "Preview deployed"
<span class="muted">Comment added to SITE-12.</span></code></pre><p class="simulator-caption">Use the same workflow in a shell script or coding agent.</p></section></div></section>`)
	}
	return template.HTML(`<section class="simulator" id="simulateur" aria-labelledby="simulateur-title"><div class="simulator-copy"><p class="eyebrow">APERÇU INTERACTIF</p><h2 id="simulateur-title">Voyez une journée de travail,<br>pas seulement des commandes.</h2><p>Explorez quatre instants d’un projet. Cette simulation locale ne crée aucun compte, n’appelle pas l’API et n’enregistre rien.</p><div class="simulator-tabs" role="tablist" aria-label="Parcours HTB"><button type="button" role="tab" id="simulator-tab-create" aria-selected="true" aria-controls="simulator-create" data-simulator-tab="simulator-create">Créer</button><button type="button" role="tab" id="simulator-tab-track" aria-selected="false" aria-controls="simulator-track" tabindex="-1" data-simulator-tab="simulator-track">Suivre</button><button type="button" role="tab" id="simulator-tab-share" aria-selected="false" aria-controls="simulator-share" tabindex="-1" data-simulator-tab="simulator-share">Partager</button><button type="button" role="tab" id="simulator-tab-automate" aria-selected="false" aria-controls="simulator-automate" tabindex="-1" data-simulator-tab="simulator-automate">Automatiser</button></div></div><div class="simulator-output"><section class="terminal simulator-panel" id="simulator-create" role="tabpanel" aria-labelledby="simulator-tab-create"><div class="terminal-bar"><span></span><span></span><span></span><code>~/site</code></div><pre><code><b>$</b> htb ticket create --title "Publier les tarifs"
<span class="success">Ticket SITE-12 créé — open.</span>

<b>$</b> htb ticket create --type technical_task \
  --parent SITE-12 --title "Ajouter le tableau comparatif"
<span class="muted">Ticket SITE-13 créé — open.</span></code></pre><p class="simulator-caption">Une US et son travail technique restent liés dès la première commande.</p></section><section class="terminal simulator-panel" id="simulator-track" role="tabpanel" aria-labelledby="simulator-tab-track" hidden><div class="terminal-bar"><span></span><span></span><span></span><code>~/site</code></div><pre><code><b>$</b> htb project status
<span class="success">* SITE — Site web</span>
  User stories  <span class="success">[██████░░░░]</span> 3/5 (60%)
  Tickets       <span class="success">[███████░░░]</span> 7/9 (77%)
  open 1 · in_progress 1 · review 0 · blocked 0 · done 7</code></pre><p class="simulator-caption">Voyez l’avancement sans fabriquer de rapport à la main.</p></section><section class="terminal simulator-panel" id="simulator-share" role="tabpanel" aria-labelledby="simulator-tab-share" hidden><div class="terminal-bar"><span></span><span></span><span></span><code>~/site</code></div><pre><code><b>$</b> htb invite create --role write
<span class="success">Code d’invitation créé : bramble-forest</span>
À partager : htb invite accept bramble-forest

<b>$</b> htb ticket claim SITE-13
<span class="muted">Ticket SITE-13 réclamé.</span></code></pre><p class="simulator-caption">Donnez le bon accès, puis rendez visible la responsabilité.</p></section><section class="terminal simulator-panel" id="simulator-automate" role="tabpanel" aria-labelledby="simulator-tab-automate" hidden><div class="terminal-bar"><span></span><span></span><span></span><code>~/site</code></div><pre><code><b>$</b> htb ticket list --json
[
  {"ref":"SITE-12","status":"in_progress"}
]

<b>$</b> htb ticket comment SITE-12 "Prévisualisation déployée"
<span class="muted">Commentaire ajouté à SITE-12.</span></code></pre><p class="simulator-caption">Utilisez le même flux dans un script shell ou un agent de code.</p></section></div></section>`)
}

func legalNoticeBody(language string) template.HTML {
	if language == "en" {
		return template.HTML(`<article class="legal"><p class="eyebrow">LEGAL NOTICE</p><h1>Legal notice</h1><h2>Publisher</h2><p>HTB (Headless Ticket Board) is published by Kazer Lelutin. Contact: <a href="mailto:b@bouteiller.contact">b@bouteiller.contact</a>.</p><h2>Hosting</h2><p>The service is deployed by Ben-to on a CapRover-managed infrastructure. The service domain is <a href="https://htb.ben-to.fr">htb.ben-to.fr</a>.</p><h2>Intellectual property</h2><p>The HTB name, source code, content, and visual identity may not be reproduced outside the applicable rights and licences without the publisher’s prior permission.</p><h2>Liability</h2><p>The publisher makes reasonable efforts to keep the service available and information accurate, but cannot guarantee uninterrupted or error-free operation.</p></article>`)
	}
	return template.HTML(`<article class="legal"><p class="eyebrow">MENTIONS LÉGALES</p><h1>Mentions légales</h1><h2>Éditeur</h2><p>HTB (Headless Ticket Board) est édité par Kazer Lelutin. Contact : <a href="mailto:b@bouteiller.contact">b@bouteiller.contact</a>.</p><h2>Hébergement</h2><p>Le service est déployé par Ben-to sur une infrastructure administrée avec CapRover. Le domaine du service est <a href="https://htb.ben-to.fr">htb.ben-to.fr</a>.</p><h2>Propriété intellectuelle</h2><p>Le nom HTB, le code source, les contenus et l’identité visuelle ne peuvent être reproduits hors des droits et licences applicables sans l’autorisation préalable de l’éditeur.</p><h2>Responsabilité</h2><p>L’éditeur met en œuvre des moyens raisonnables pour assurer l’exactitude des informations et la disponibilité du service, sans garantir une disponibilité ininterrompue ni l’absence d’erreur.</p></article>`)
}

func termsBody(language string) template.HTML {
	if language == "en" {
		return template.HTML(`<article class="legal"><p class="eyebrow">TERMS OF SERVICE</p><h1>Terms of service</h1><h2>Purpose</h2><p>HTB is a terminal-first ticket tracking service. These terms govern use of the public website, the API, and the command-line client.</p><h2>Access and accounts</h2><p>Authentication is provided by Zitadel. You are responsible for safeguarding your account and for activity performed with it. Project administrators are responsible for the people they invite and the permissions they grant.</p><h2>Acceptable use</h2><p>Do not use HTB to violate the law, interfere with the service, access projects without authorisation, or submit unlawful content. Do not attempt to bypass technical or future usage limits.</p><h2>Your content</h2><p>You remain responsible for tickets, comments, labels, and project information you submit. Do not put secrets or personal data in tickets unless it is necessary and you have a lawful basis to do so.</p><h2>Service evolution</h2><p>HTB may evolve, including future plan and project-limit features. Material changes to these terms will be published on this page.</p><h2>Contact</h2><p>For a question about these terms, email <a href="mailto:b@bouteiller.contact">b@bouteiller.contact</a>.</p></article>`)
	}
	return template.HTML(`<article class="legal"><p class="eyebrow">CONDITIONS GÉNÉRALES D’UTILISATION</p><h1>CGU</h1><h2>Objet</h2><p>HTB est un service de suivi de tickets pensé pour le terminal. Les présentes CGU encadrent l’utilisation du site public, de l’API et du client en ligne de commande.</p><h2>Accès et comptes</h2><p>L’authentification est fournie par Zitadel. Vous êtes responsable de la protection de votre compte et des activités effectuées avec celui-ci. Les administrateurs de projet sont responsables des personnes invitées et des droits qu’ils accordent.</p><h2>Usage autorisé</h2><p>Il est interdit d’utiliser HTB pour enfreindre la loi, perturber le service, accéder à un projet sans autorisation ou déposer des contenus illicites. Il est également interdit de contourner des limites techniques ou d’usage présentes ou futures.</p><h2>Vos contenus</h2><p>Vous restez responsable des tickets, commentaires, libellés et informations de projet que vous saisissez. N’inscrivez pas de secrets ou de données personnelles dans les tickets sans nécessité et sans base légale.</p><h2>Évolution du service</h2><p>HTB peut évoluer, notamment avec de futures offres et limites de projets. Toute modification substantielle de ces CGU sera publiée sur cette page.</p><h2>Contact</h2><p>Pour toute question relative à ces CGU : <a href="mailto:b@bouteiller.contact">b@bouteiller.contact</a>.</p></article>`)
}

func privacyBody(language string) template.HTML {
	if language == "en" {
		return template.HTML(`<article class="legal"><p class="eyebrow">PRIVACY AND COOKIES</p><h1>Privacy and cookies</h1><h2>Data processed</h2><p>HTB stores the Zitadel subject identifier, name and email needed to identify an account. It also stores project memberships and the content submitted through the service: projects, tickets, comments, labels, invitations, and audit history.</p><h2>Why and for how long</h2><p>This data is used to authenticate you, apply project permissions, provide the service, and preserve its traceability. It is kept for the life of the account or project, unless a longer legal retention obligation applies. To request access, correction, or deletion, contact the publisher.</p><h2>Analytics preference</h2><p>With your explicit consent, HTB loads the analytics script from <code>analytics.ben-to.fr</code>. Your choice is stored only in your browser’s local storage under <code>htb.analytics-consent</code>. Refusing prevents the script from being loaded.</p><h2>Managing your choice</h2><p>Use “Analytics preferences” in the footer at any time to change your choice. HTB does not load advertising or social-media trackers.</p></article>`)
	}
	return template.HTML(`<article class="legal"><p class="eyebrow">CONFIDENTIALITÉ ET COOKIES</p><h1>Confidentialité et cookies</h1><h2>Données traitées</h2><p>HTB enregistre l’identifiant Zitadel, le nom et l’e-mail nécessaires à l’identification d’un compte. Le service enregistre également les appartenances aux projets et les contenus saisis : projets, tickets, commentaires, libellés, invitations et historique d’audit.</p><h2>Finalités et durée</h2><p>Ces données servent à vous authentifier, appliquer les droits de projet, fournir le service et préserver sa traçabilité. Elles sont conservées pendant la durée de vie du compte ou du projet, sauf obligation légale de conservation plus longue. Pour demander l’accès, la rectification ou la suppression de vos données, contactez l’éditeur.</p><h2>Préférence de mesure</h2><p>Avec votre consentement explicite, HTB charge le script de mesure d’audience de <code>analytics.ben-to.fr</code>. Votre choix est conservé uniquement dans le stockage local de votre navigateur sous la clé <code>htb.analytics-consent</code>. Un refus empêche le chargement du script.</p><h2>Gérer votre choix</h2><p>Utilisez à tout moment « Préférences de mesure » dans le pied de page pour modifier votre choix. HTB ne charge pas de traceur publicitaire ou de réseau social.</p></article>`)
}

func downloadsBody(language, installationURL, releaseURL string) template.HTML {
	installationURL, releaseURL = html.EscapeString(installationURL), html.EscapeString(releaseURL)
	var out strings.Builder
	fmt.Fprintf(&out, `<article class="downloads"><p class="eyebrow">%s</p><h1>%s</h1><p class="lede">%s</p><pre class="install"><code>curl -fsSL %s | sh</code></pre><p><a class="text-link" href="%s">%s <span aria-hidden="true">→</span></a></p><h2>%s</h2><table><caption>%s</caption><thead><tr><th>%s</th><th>%s</th><th>%s</th></tr></thead><tbody>`,
		html.EscapeString(localized(language, "INSTALLATION", "INSTALLATION")), html.EscapeString(localized(language, "Télécharger HTB", "Download HTB")), html.EscapeString(localized(language, "Installez la CLI Linux, puis connectez-vous avec htb auth login.", "Install the Linux CLI, then sign in with htb auth login.")), installationURL, releaseURL, html.EscapeString(localized(language, "Archives, sommes de contrôle et releases", "Archives, checksums, and releases")), html.EscapeString(localized(language, "Guide des commandes", "Command guide")), html.EscapeString(localized(language, "Toutes les commandes disponibles", "All supported commands")), html.EscapeString(localized(language, "Catégorie", "Area")), html.EscapeString(localized(language, "Commande", "Command")), html.EscapeString(localized(language, "Utilisation", "How to use it")))
	for _, command := range downloadCommands {
		fmt.Fprintf(&out, "<tr><td>%s</td><td><code>%s</code></td><td>%s</td></tr>", html.EscapeString(command.Group), html.EscapeString(command.Syntax), html.EscapeString(command.Description))
	}
	out.WriteString("</tbody></table></article>")
	return template.HTML(out.String())
}
