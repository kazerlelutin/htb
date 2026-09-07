package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var version = "dev"

type config struct {
	Server, Token, RefreshToken, Issuer, ClientID, Audience, CurrentProject string
	ExpiresAt                                                               time.Time
}

type projectView struct {
	Key, Name, Description string
	Archived               bool
}

type featureView struct {
	Key, Name, Description string
	DueDate                *time.Time `json:"due_date"`
}

type ticketView struct {
	Ref          string   `json:"ref"`
	Project      string   `json:"project"`
	Type         string   `json:"type"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Status       string   `json:"status"`
	Priority     string   `json:"priority"`
	ParentRef    *string  `json:"parent_ref"`
	RelatedRef   *string  `json:"related_ref"`
	FeatureKey   *string  `json:"feature_key"`
	Labels       []string `json:"labels"`
	Version      int      `json:"version"`
	ChildCount   int      `json:"child_count"`
	DoneChildren int      `json:"done_children"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}
	if os.Args[1] == "help" {
		printHelp(os.Args[2:])
		return
	}
	if isHelpFlag(os.Args[1]) {
		usage()
		return
	}
	if len(os.Args) > 2 && isHelpFlag(os.Args[len(os.Args)-1]) {
		printHelp(os.Args[1 : len(os.Args)-1])
		return
	}
	var err error
	switch os.Args[1] {
	case "version":
		fmt.Println(version)
	case "config":
		err = configCommand(os.Args[2:])
	case "auth":
		err = authCommand(os.Args[2:])
	case "project":
		err = projectCommand(os.Args[2:])
	case "feature":
		err = featureCommand(os.Args[2:])
	case "ticket":
		err = ticketCommand(os.Args[2:])
	case "invite":
		err = inviteCommand(os.Args[2:])
	default:
		usage()
		err = errors.New("unknown command")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "htb:", err)
		os.Exit(1)
	}
}

func usage() {
	printHelp(nil)
}

func isHelpFlag(value string) bool { return value == "--help" || value == "-h" }

func printHelp(parts []string) {
	command := strings.Join(parts, " ")
	switch command {
	case "", "htb":
		fmt.Print(`HTB — Headless Ticket Board

Usage: htb <commande> [options]

Premiers pas:
  htb config set-server URL       Configure le serveur HTB
  htb auth login                  Ouvre la connexion Zitadel
  htb auth status                 Affiche la connexion et le projet courant

Commandes:
  project list | create | use     Gérer les projets
  feature create                  Créer une fonctionnalité de roadmap
  ticket create | list | show     Créer, parcourir ou consulter les tickets
  ticket update | comment         Modifier ou commenter un ticket
  ticket claim | release          Prendre ou libérer un ticket
  ticket versions | restore       Consulter ou restaurer l'historique
  invite create | accept          Inviter ou rejoindre un projet

Utilisez « htb help ticket create » ou « htb ticket create --help » pour le détail.
`)
	case "config":
		fmt.Println("Usage: htb config set-server URL\n\nExemple: htb config set-server https://tickets.example.org")
	case "auth":
		fmt.Println("Usage: htb auth {login|status}\n\nlogin ouvre Zitadel dans le navigateur ; status affiche l'identité et le projet courant.")
	case "project":
		fmt.Println("Usage:\n  htb project list\n  htb project use KEY\n  htb project create --key KEY --name NAME [--description TEXTE]")
	case "feature", "feature create":
		fmt.Println("Usage: htb feature create --key KEY --name NAME [--project KEY] [--description TEXTE] [--due-date YYYY-MM-DD]\n\nExemple: htb feature create --key newsletter --name Newsletter --due-date 2026-09-30")
	case "ticket":
		fmt.Println("Usage:\n  htb ticket create [options]\n  htb ticket list [--project KEY] [--feature KEY] [--tree] [--json|--csv]\n  htb ticket show REF\n  htb ticket update --version N [options] REF\n  htb ticket comment REF TEXTE\n  htb ticket claim|release REF\n  htb ticket versions REF\n  htb ticket restore --version N REF REVISION")
	case "ticket create":
		fmt.Println("Usage: htb ticket create --title TITRE [--type user_story|technical_task|bug|incident] [--project KEY] [--parent REF] [--feature KEY] [--priority low|normal|high|urgent] [--label TAG]\n\nUne tâche technique requiert --parent US-REF. Les labels peuvent être répétés.")
	case "ticket update":
		fmt.Println("Usage: htb ticket update --version N [--title TITRE] [--description TEXTE] [--status open|in_progress|review|blocked|done] [--priority PRIORITÉ] [--feature KEY] REF\n\nLa version affichée par « htb ticket show REF » évite d'écraser une modification concurrente. L'état d'une US est calculé depuis ses tâches.")
	case "ticket list":
		fmt.Println("Usage: htb ticket list [--project KEY] [--feature KEY] [--tree] [--json|--csv]\n\nSans option, la sortie est adaptée au terminal. --json et --csv sont destinés aux scripts.")
	case "invite":
		fmt.Println("Usage:\n  htb invite create [--project KEY] [--role read|write|admin] [--expires-at RFC3339]\n  htb invite accept CODE")
	default:
		fmt.Printf("Aide indisponible pour « htb %s ». Lancez « htb help ».\n", command)
	}
}
func configCommand(args []string) error {
	if len(args) == 2 && args[0] == "set-server" {
		c, _ := load()
		c.Server = strings.TrimRight(args[1], "/")
		return save(c)
	}
	return errors.New("usage: htb config set-server URL")
}
func authCommand(args []string) error {
	if len(args) == 1 && args[0] == "status" {
		return authStatus()
	}
	if len(args) > 0 && args[0] == "login" {
		return deviceLogin(args[1:])
	}
	return errors.New("usage: htb auth {login|status}")
}
func deviceLogin(args []string) error {
	fs := flag.NewFlagSet("auth login", flag.ContinueOnError)
	issuer := fs.String("issuer", "", "Zitadel issuer")
	clientID := fs.String("client-id", "", "Zitadel Device Code client ID")
	audience := fs.String("audience", "", "HTB Zitadel project ID")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, _ := load()
	if (*issuer == "" || *clientID == "" || *audience == "") && c.Server != "" {
		var remote struct {
			Issuer   string `json:"issuer"`
			ClientID string `json:"client_id"`
			Audience string `json:"audience"`
		}
		if err := getJSON(c.Server+"/auth/device-config", &remote); err == nil {
			if *issuer == "" {
				*issuer = remote.Issuer
			}
			if *clientID == "" {
				*clientID = remote.ClientID
			}
			if *audience == "" {
				*audience = remote.Audience
			}
		}
	}
	if *issuer == "" {
		*issuer = c.Issuer
	}
	if *clientID == "" {
		*clientID = c.ClientID
	}
	if *audience == "" {
		*audience = c.Audience
	}
	if *issuer == "" || *clientID == "" || *audience == "" {
		return errors.New("--issuer, --client-id and --audience are required on first login")
	}
	var discovery struct {
		DeviceAuthorizationEndpoint string `json:"device_authorization_endpoint"`
		TokenEndpoint               string `json:"token_endpoint"`
	}
	if err := getJSON(strings.TrimRight(*issuer, "/")+"/.well-known/openid-configuration", &discovery); err != nil {
		return err
	}
	scopes := deviceScopes(*audience)
	resp, err := http.PostForm(discovery.DeviceAuthorizationEndpoint, url.Values{"client_id": {*clientID}, "scope": {scopes}})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var device struct {
		DeviceCode              string `json:"device_code"`
		VerificationURIComplete string `json:"verification_uri_complete"`
		VerificationURI         string `json:"verification_uri"`
		UserCode                string `json:"user_code"`
		Interval                int    `json:"interval"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&device); err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return errors.New("device authorization failed")
	}
	if device.VerificationURIComplete != "" {
		if err := openBrowser(device.VerificationURIComplete); err == nil {
			fmt.Fprintln(os.Stderr, "Ouverture de la page de connexion Zitadel…")
		} else {
			fmt.Fprintln(os.Stderr, "Ouvrez cette adresse :", device.VerificationURIComplete)
		}
	} else {
		fmt.Fprintln(os.Stderr, "Open:", device.VerificationURI, "and enter", device.UserCode)
	}
	if device.Interval < 1 {
		device.Interval = 5
	}
	for {
		time.Sleep(time.Duration(device.Interval) * time.Second)
		tokenResponse, err := http.PostForm(discovery.TokenEndpoint, url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}, "device_code": {device.DeviceCode}, "client_id": {*clientID}})
		if err != nil {
			return err
		}
		var token struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int    `json:"expires_in"`
			Error        string `json:"error"`
		}
		err = json.NewDecoder(tokenResponse.Body).Decode(&token)
		tokenResponse.Body.Close()
		if err != nil {
			return err
		}
		if token.AccessToken != "" {
			c.Token, c.RefreshToken, c.Issuer, c.ClientID, c.Audience = token.AccessToken, token.RefreshToken, *issuer, *clientID, *audience
			c.ExpiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
			if err := save(c); err != nil {
				return err
			}
			return authStatus()
		}
		if token.Error != "authorization_pending" && token.Error != "slow_down" {
			return fmt.Errorf("login failed: %s", token.Error)
		}
		if token.Error == "slow_down" {
			device.Interval += 5
		}
	}
}
func openBrowser(rawURL string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", rawURL)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		command = exec.Command("xdg-open", rawURL)
	}
	return command.Start()
}
func getJSON(raw string, target any) error {
	response, err := http.Get(raw)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return fmt.Errorf("request to %s failed: %s", raw, response.Status)
	}
	return json.NewDecoder(response.Body).Decode(target)
}
func deviceScopes(audience string) string {
	return strings.Join([]string{
		"openid", "profile", "offline_access",
		"urn:zitadel:iam:org:project:id:" + audience + ":aud",
		"urn:zitadel:iam:org:projects:roles",
	}, " ")
}
func authStatus() error {
	c, err := load()
	if err != nil {
		return err
	}
	if c.Server == "" || (c.Token == "" && os.Getenv("HTB_TOKEN") == "") {
		return errors.New("not connected; run htb auth login")
	}
	var me struct {
		Subject    string `json:"subject"`
		Superadmin bool   `json:"superadmin"`
	}
	if err := call("GET", "/api/v1/me", nil, &me); err != nil {
		return err
	}
	var response struct {
		Projects []struct {
			Key  string `json:"key"`
			Name string `json:"name"`
		} `json:"projects"`
	}
	if err := call("GET", "/api/v1/projects", nil, &response); err != nil {
		return err
	}
	label := me.Subject
	if me.Superadmin {
		label += " (superadmin)"
	}
	fmt.Printf("Connecté : %s\nServeur : %s\n", label, c.Server)
	if len(response.Projects) == 0 {
		fmt.Println("Aucun projet accessible. Créez-en un avec ‘htb project create’ ou rejoignez-en un avec ‘htb invite accept CODE’. ")
		return nil
	}
	fmt.Println("Projets :")
	for _, project := range response.Projects {
		marker := " "
		if project.Key == c.CurrentProject {
			marker = "*"
		}
		fmt.Printf("%s %s — %s\n", marker, project.Key, project.Name)
	}
	if c.CurrentProject == "" {
		fmt.Printf("Projet courant non défini. Utilisez ‘htb project use %s’.\n", response.Projects[0].Key)
	}
	return nil
}
func currentProject() (string, error) {
	c, err := load()
	if err != nil {
		return "", err
	}
	if c.CurrentProject == "" {
		return "", errors.New("no current project; run htb project use KEY")
	}
	return c.CurrentProject, nil
}
func refreshAccessToken(c *config) error {
	if c.RefreshToken == "" || c.Issuer == "" || c.ClientID == "" {
		return errors.New("session expired; run htb auth login")
	}
	var discovery struct {
		TokenEndpoint string `json:"token_endpoint"`
	}
	if err := getJSON(strings.TrimRight(c.Issuer, "/")+"/.well-known/openid-configuration", &discovery); err != nil {
		return err
	}
	response, err := http.PostForm(discovery.TokenEndpoint, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {c.RefreshToken}, "client_id": {c.ClientID}})
	if err != nil {
		return err
	}
	defer response.Body.Close()
	var token struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Error        string `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&token); err != nil {
		return err
	}
	if response.StatusCode >= 300 || token.AccessToken == "" {
		return fmt.Errorf("session refresh failed: %s", token.Error)
	}
	c.Token = token.AccessToken
	if token.RefreshToken != "" {
		c.RefreshToken = token.RefreshToken
	}
	c.ExpiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	return save(*c)
}
func projectCommand(args []string) error {
	if len(args) == 2 && args[0] == "use" {
		var response struct {
			Projects []struct {
				Key string `json:"key"`
			} `json:"projects"`
		}
		if err := call("GET", "/api/v1/projects", nil, &response); err != nil {
			return err
		}
		wanted := strings.ToUpper(args[1])
		for _, project := range response.Projects {
			if project.Key == wanted {
				c, err := load()
				if err != nil {
					return err
				}
				c.CurrentProject = wanted
				if err := save(c); err != nil {
					return err
				}
				fmt.Println("Projet courant :", wanted)
				return nil
			}
		}
		return fmt.Errorf("project %s is not accessible", wanted)
	}
	if len(args) == 1 && args[0] == "list" {
		var response struct {
			Projects []projectView `json:"projects"`
		}
		if err := call("GET", "/api/v1/projects", nil, &response); err != nil {
			return err
		}
		if len(response.Projects) == 0 {
			fmt.Println("Aucun projet accessible.")
			return nil
		}
		c, err := load()
		if err != nil {
			return err
		}
		fmt.Println("Projets accessibles :")
		for _, project := range response.Projects {
			marker := " "
			if project.Key == c.CurrentProject {
				marker = "*"
			}
			fmt.Printf("%s %s — %s%s\n", marker, project.Key, project.Name, archivedLabel(project.Archived))
		}
		return nil
	}
	if len(args) == 0 || args[0] != "create" {
		return errors.New("usage: htb project {list|use KEY|create --key KEY --name NAME}")
	}
	fs := flag.NewFlagSet("project create", flag.ContinueOnError)
	key := fs.String("key", "", "project key")
	name := fs.String("name", "", "name")
	description := fs.String("description", "", "description")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	var created projectView
	if err := call("POST", "/api/v1/projects", map[string]string{"key": *key, "name": *name, "description": *description}, &created); err != nil {
		return err
	}
	c, err := load()
	if err != nil {
		return err
	}
	c.CurrentProject = strings.ToUpper(*key)
	if err := save(c); err != nil {
		return err
	}
	fmt.Printf("Projet %s créé : %s. Il est maintenant le projet courant.\n", created.Key, created.Name)
	return nil
}

func featureCommand(args []string) error {
	if len(args) == 0 || args[0] != "create" {
		return errors.New("usage: htb feature create --project KEY --key KEY --name NAME")
	}
	fs := flag.NewFlagSet("feature create", flag.ContinueOnError)
	project := fs.String("project", "", "project")
	key := fs.String("key", "", "feature key")
	name := fs.String("name", "", "name")
	description := fs.String("description", "", "description")
	dueDate := fs.String("due-date", "", "due date (YYYY-MM-DD)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *project == "" {
		var err error
		*project, err = currentProject()
		if err != nil {
			return err
		}
	}
	var dueAt *time.Time
	if *dueDate != "" {
		parsed, err := time.Parse("2006-01-02", *dueDate)
		if err != nil {
			return errors.New("--due-date must use YYYY-MM-DD")
		}
		dueAt = &parsed
	}
	var created featureView
	if err := call("POST", "/api/v1/projects/"+*project+"/features", map[string]any{"key": *key, "name": *name, "description": *description, "due_date": dueAt}, &created); err != nil {
		return err
	}
	due := ""
	if created.DueDate != nil {
		due = " Échéance : " + created.DueDate.Format("2006-01-02") + "."
	}
	fmt.Printf("Fonctionnalité %s créée : %s.%s\n", created.Key, created.Name, due)
	return nil
}
func ticketCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: htb ticket {create|list|show|update|versions}")
	}
	switch args[0] {
	case "create":
		return ticketCreate(args[1:])
	case "list":
		return ticketList(args[1:])
	case "show":
		return ticketShow(args[1:])
	case "update":
		return ticketUpdate(args[1:])
	case "versions":
		return ticketVersions(args[1:])
	case "restore":
		return ticketRestore(args[1:])
	case "comment":
		return ticketComment(args[1:])
	case "claim":
		return ticketAction(args[1:], "claim")
	case "release":
		return ticketAction(args[1:], "release")
	default:
		return errors.New("unknown ticket command")
	}
}
func ticketComment(args []string) error {
	if len(args) != 2 {
		return errors.New("usage: htb ticket comment REF TEXT")
	}
	if err := call("POST", "/api/v1/tickets/"+args[0]+"/comments", map[string]string{"body": args[1]}, &struct{}{}); err != nil {
		return err
	}
	fmt.Printf("Commentaire ajouté à %s.\n", strings.ToUpper(args[0]))
	return nil
}
func ticketAction(args []string, action string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: htb ticket %s REF", action)
	}
	if action == "release" {
		if err := call("POST", "/api/v1/tickets/"+args[0]+"/"+action, map[string]any{}, &struct{}{}); err != nil {
			return err
		}
		fmt.Printf("Ticket %s libéré.\n", strings.ToUpper(args[0]))
		return nil
	}
	var ticket ticketView
	if err := call("POST", "/api/v1/tickets/"+args[0]+"/"+action, map[string]any{}, &ticket); err != nil {
		return err
	}
	fmt.Printf("Ticket %s pris en charge.\n", ticket.Ref)
	return nil
}
func ticketCreate(args []string) error {
	fs := flag.NewFlagSet("ticket create", flag.ContinueOnError)
	project := fs.String("project", "", "project")
	kind := fs.String("type", "user_story", "user_story|technical_task|bug|incident")
	parent := fs.String("parent", "", "parent ref")
	related := fs.String("related", "", "related ref")
	feature := fs.String("feature", "", "feature key")
	title := fs.String("title", "", "title")
	description := fs.String("description", "", "description")
	priority := fs.String("priority", "normal", "priority")
	labels := multiFlag{}
	fs.Var(&labels, "label", "label (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *project == "" {
		var err error
		*project, err = currentProject()
		if err != nil {
			return err
		}
	}
	var ticket ticketView
	if err := call("POST", "/api/v1/tickets", map[string]any{"project": *project, "type": *kind, "parent_ref": *parent, "related_ref": *related, "feature_key": *feature, "title": *title, "description": *description, "priority": *priority, "labels": []string(labels)}, &ticket); err != nil {
		return err
	}
	fmt.Printf("Ticket %s créé — %s [%s].\n", ticket.Ref, ticket.Title, ticket.Status)
	return nil
}
func ticketList(args []string) error {
	fs := flag.NewFlagSet("ticket list", flag.ContinueOnError)
	project := fs.String("project", "", "project")
	tree := fs.Bool("tree", false, "include child tickets")
	feature := fs.String("feature", "", "feature key")
	csvOutput := fs.Bool("csv", false, "csv output")
	jsonOutput := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *project == "" {
		var err error
		*project, err = currentProject()
		if err != nil {
			return err
		}
	}
	query := url.Values{"project": {*project}, "tree": {fmt.Sprint(*tree)}}
	if *feature != "" {
		query.Set("feature", *feature)
	}
	path := "/api/v1/tickets?" + query.Encode()
	if *csvOutput || *jsonOutput {
		var out any
		if err := call("GET", path, nil, &out); err != nil {
			return err
		}
		if *csvOutput {
			return renderCSV(out)
		}
		return printValue(out)
	}
	var response struct {
		Tickets []ticketView `json:"tickets"`
	}
	if err := call("GET", path, nil, &response); err != nil {
		return err
	}
	if len(response.Tickets) == 0 {
		fmt.Printf("Aucun ticket dans le projet %s.\n", strings.ToUpper(*project))
		return nil
	}
	heading := "Tickets — " + strings.ToUpper(*project)
	if *feature != "" {
		heading += " / fonctionnalité " + *feature
	}
	fmt.Println(heading)
	fmt.Println("REF          ÉTAT           PROGRESSION       TYPE              TITRE")
	for _, ticket := range response.Tickets {
		indent := ""
		if ticket.ParentRef != nil {
			indent = "↳ "
		}
		fmt.Printf("%-12s %s %-17s %-17s %s%s\n", ticket.Ref, paddedStatus(ticket.Status), ticketProgress(ticket), ticket.Type, indent, ticket.Title)
	}
	return nil
}
func ticketShow(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: htb ticket show REF")
	}
	var ticket ticketView
	if err := call("GET", "/api/v1/tickets/"+args[0], nil, &ticket); err != nil {
		return err
	}
	printTicket(ticket)
	return nil
}
func ticketUpdate(args []string) error {
	fs := flag.NewFlagSet("ticket update", flag.ContinueOnError)
	expected := fs.Int("version", 0, "current ticket version")
	title := fs.String("title", "", "title")
	description := fs.String("description", "", "description")
	status := fs.String("status", "", "status")
	priority := fs.String("priority", "", "priority")
	feature := fs.String("feature", "", "feature key")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: htb ticket update [flags] REF")
	}
	body := map[string]any{"expected_version": *expected}
	if *title != "" {
		body["title"] = *title
	}
	if *description != "" {
		body["description"] = *description
	}
	if *status != "" {
		body["status"] = *status
	}
	if *priority != "" {
		body["priority"] = *priority
	}
	if *feature != "" {
		body["feature_key"] = *feature
	}
	var ticket ticketView
	if err := call("PATCH", "/api/v1/tickets/"+fs.Arg(0), body, &ticket); err != nil {
		return err
	}
	fmt.Printf("Ticket %s mis à jour (version %d).\n", ticket.Ref, ticket.Version)
	return nil
}
func ticketVersions(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: htb ticket versions REF")
	}
	var out any
	if err := call("GET", "/api/v1/tickets/"+args[0]+"/versions", nil, &out); err != nil {
		return err
	}
	return printValue(out)
}
func ticketRestore(args []string) error {
	fs := flag.NewFlagSet("ticket restore", flag.ContinueOnError)
	current := fs.Int("version", 0, "current ticket version")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return errors.New("usage: htb ticket restore --version CURRENT REF REVISION")
	}
	var ticket ticketView
	if err := call("POST", "/api/v1/tickets/"+fs.Arg(0)+"/versions/"+fs.Arg(1)+"/restore", map[string]int{"expected_version": *current}, &ticket); err != nil {
		return err
	}
	fmt.Printf("Ticket %s restauré (version %d).\n", ticket.Ref, ticket.Version)
	return nil
}
func inviteCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: htb invite {create|accept}")
	}
	if args[0] == "accept" && len(args) == 2 {
		if err := call("POST", "/api/v1/invitations/"+args[1]+"/accept", map[string]any{}, &struct{}{}); err != nil {
			return err
		}
		fmt.Println("Invitation acceptée. Le projet est maintenant accessible.")
		return nil
	}
	if args[0] == "create" {
		fs := flag.NewFlagSet("invite create", flag.ContinueOnError)
		project := fs.String("project", "", "project")
		role := fs.String("role", "read", "role")
		expires := fs.String("expires-at", "", "RFC3339 expiry")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *project == "" {
			var err error
			*project, err = currentProject()
			if err != nil {
				return err
			}
		}
		var response struct {
			Code string `json:"code"`
		}
		if err := call("POST", "/api/v1/invitations", map[string]string{"project": *project, "role": *role, "expires_at": *expires}, &response); err != nil {
			return err
		}
		fmt.Printf("Code d'invitation créé : %s\nPartagez-le avec : htb invite accept %s\n", response.Code, response.Code)
		return nil
	}
	return errors.New("usage: htb invite create --project KEY --role read --expires-at RFC3339 | htb invite accept CODE")
}

func call(method, path string, input any, output any) error {
	c, err := load()
	if err != nil {
		return err
	}
	token := os.Getenv("HTB_TOKEN")
	if token == "" {
		if !c.ExpiresAt.IsZero() && time.Now().After(c.ExpiresAt.Add(-30*time.Second)) {
			if err := refreshAccessToken(&c); err != nil {
				return err
			}
		}
		token = c.Token
	}
	if c.Server == "" {
		return errors.New("server is not configured")
	}
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.Server+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s", strings.TrimSpace(string(data)))
	}
	if output != nil {
		return json.Unmarshal(data, output)
	}
	_, err = os.Stdout.Write(data)
	return err
}
func configPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "htb", "config.json"), nil
}
func load() (config, error) {
	path, err := configPath()
	if err != nil {
		return config{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return config{}, nil
	}
	if err != nil {
		return config{}, err
	}
	var c config
	return c, json.Unmarshal(data, &c)
}
func save(c config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }
func printValue(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func archivedLabel(archived bool) string {
	if archived {
		return " (archivé)"
	}
	return ""
}

func printTicket(ticket ticketView) {
	fmt.Printf("%s — %s\n", ticket.Ref, ticket.Title)
	fmt.Printf("Projet : %s | Type : %s | État : %s | Priorité : %s | Version : %d\n", ticket.Project, ticket.Type, styledStatus(ticket.Status), ticket.Priority, ticket.Version)
	if progress := ticketProgress(ticket); progress != "—" {
		fmt.Println("Progression :", progress)
	}
	if ticket.ParentRef != nil {
		fmt.Println("Ticket parent :", *ticket.ParentRef)
	}
	if ticket.RelatedRef != nil {
		fmt.Println("Ticket lié :", *ticket.RelatedRef)
	}
	if ticket.FeatureKey != nil {
		fmt.Println("Fonctionnalité :", *ticket.FeatureKey)
	}
	if len(ticket.Labels) > 0 {
		fmt.Println("Tags :", strings.Join(ticket.Labels, ", "))
	}
	if ticket.Description != "" {
		fmt.Printf("\n%s\n", ticket.Description)
	}
}

func ticketProgress(ticket ticketView) string {
	if ticket.Type != "user_story" || ticket.ChildCount == 0 {
		return "—"
	}
	percent := ticket.DoneChildren * 100 / ticket.ChildCount
	return fmt.Sprintf("%d/%d tâches (%d%%)", ticket.DoneChildren, ticket.ChildCount, percent)
}

func styledStatus(status string) string {
	if !colorsEnabled() {
		return status
	}
	color := map[string]string{
		"open":        "\x1b[90m",
		"in_progress": "\x1b[36m",
		"review":      "\x1b[33m",
		"blocked":     "\x1b[31m",
		"done":        "\x1b[32m",
	}[status]
	if color == "" {
		return status
	}
	return color + status + "\x1b[0m"
}

func paddedStatus(status string) string {
	return styledStatus(status) + strings.Repeat(" ", max(0, 14-len(status)))
}

func colorsEnabled() bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("HTB_COLOR") == "never" {
		return false
	}
	if os.Getenv("HTB_COLOR") == "always" {
		return true
	}
	info, err := os.Stdout.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
func renderCSV(v any) error {
	root, ok := v.(map[string]any)
	if !ok {
		return printValue(v)
	}
	tickets, ok := root["tickets"].([]any)
	if !ok {
		return printValue(v)
	}
	w := csv.NewWriter(os.Stdout)
	defer w.Flush()
	_ = w.Write([]string{"ref", "type", "status", "priority", "title"})
	for _, item := range tickets {
		ticket := item.(map[string]any)
		_ = w.Write([]string{fmt.Sprint(ticket["ref"]), fmt.Sprint(ticket["type"]), fmt.Sprint(ticket["status"]), fmt.Sprint(ticket["priority"]), fmt.Sprint(ticket["title"])})
	}
	return w.Error()
}
