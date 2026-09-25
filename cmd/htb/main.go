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
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kazerlelutin/htb/internal/domain"
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

type progressView struct {
	Total int `json:"total"`
	Done  int `json:"done"`
}

type statusCountsView struct {
	Open       int `json:"open"`
	InProgress int `json:"in_progress"`
	Review     int `json:"review"`
	Blocked    int `json:"blocked"`
	Done       int `json:"done"`
}

type projectStatusView struct {
	projectView
	UserStories progressView     `json:"user_stories"`
	Tickets     progressView     `json:"tickets"`
	Statuses    statusCountsView `json:"statuses"`
}

type featureView struct {
	Key, Name, Description string
	DueDate                *time.Time `json:"due_date"`
}
type memberView struct {
	UserID int64  `json:"user_id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Owner  bool   `json:"owner"`
}
type invitationView struct {
	ID        int64     `json:"id"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expires_at"`
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
	Published    bool     `json:"published"`
}
type commentView struct {
	Body, Author string
	CreatedAt    time.Time `json:"created_at"`
}
type activityView struct {
	Action, Actor string
	CreatedAt     time.Time `json:"created_at"`
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
	case "request":
		err = requestCommand(os.Args[2:])
	case "invite":
		err = inviteCommand(os.Args[2:])
	default:
		usage()
		err = errors.New("unknown command")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, styledError("htb:"), err)
		os.Exit(1)
	}
}

func usage() {
	printHelp(nil)
}

func isHelpFlag(value string) bool { return value == "--help" || value == "-h" }

func printHelp(parts []string) { fmt.Print(helpText(parts)) }

func helpText(parts []string) string {
	command := strings.Join(parts, " ")
	switch command {
	case "", "htb":
		return `HTB — Headless Ticket Board

Usage: htb <command> [options]

Getting started:
  htb config set-server URL       Set the HTB server
  htb auth login                  Sign in through the browser
  htb auth status                 Show connection and current project

Commands:
  version                         Show the CLI version
  project list | status | create | use | members | member     Manage projects and access
  feature create                  Create a roadmap feature
  ticket create | list | show     Create, browse, or view tickets
  ticket update | comment         Update or comment on a ticket
  ticket publish | unpublish      Control client visibility of a user story
  ticket client-comments | client-comment   Read or reply to client comments
  ticket claim | release          Claim or release a ticket
  ticket versions | restore       View or restore history
  request list | show | comments | comment | status | link   Handle client requests
  invite create | list | revoke | accept     Invite or join a project

Use "htb help ticket create" or "htb ticket create --help" for command details.
`
	case "version":
		return "Usage: htb version\n\nShow the installed CLI version.\n"
	case "config", "config set-server":
		return "Usage: htb config set-server URL\n\nSet the HTB server URL. Example: htb config set-server https://tickets.example.org\n"
	case "auth":
		return "Usage:\n  htb auth login [--issuer URL --client-id ID --audience ID]\n  htb auth status\n\nSign in or show your connection and current project.\n"
	case "auth login":
		return "Usage: htb auth login [--issuer URL --client-id ID --audience ID]\n\nOpen the browser sign-in flow. Advanced options override the server-provided connection settings.\n"
	case "auth status":
		return "Usage: htb auth status\n\nShow whether you are connected, your accessible projects, and the current project.\n"
	case "project":
		return "Usage:\n  htb project list | status | use KEY\n  htb project create --key KEY --name NAME [--description TEXT]\n  htb project members [--project KEY]\n  htb project member set-role --user ID --role read|write|admin [--project KEY]\n  htb project member remove --user ID [--project KEY]\n\nA project key is the short identifier used in commands and ticket references, for example HTB-1. Input is normalized to uppercase. It must be 2 to 20 characters, start with a letter, and contain only letters, digits, or underscores.\n"
	case "project list":
		return "Usage: htb project list\n\nList projects you can access.\n"
	case "project status":
		return "Usage: htb project status\n\nShow user-story and ticket progress, plus the status breakdown, for every project you can access.\n"
	case "project use":
		return "Usage: htb project use KEY\n\nSet the current project used by commands that do not specify --project.\n"
	case "project create":
		return "Usage: htb project create --key KEY --name NAME [--description TEXT]\n\nCreate a project and make it current. A project key is the short identifier used in commands and ticket references, for example HTB-1. Input is normalized to uppercase. It must be 2 to 20 characters, start with a letter, and contain only letters, digits, or underscores.\n"
	case "project members":
		return "Usage: htb project members [--project KEY]\n\nList project members. Administrators can use member IDs to change a role or remove access.\n"
	case "project member":
		return "Usage:\n  htb project member set-role --user ID --role read|write|admin [--project KEY]\n  htb project member remove --user ID [--project KEY]\n\nManage a non-owner project member. The project owner cannot be removed or demoted.\n"
	case "feature", "feature create":
		return "Usage: htb feature create --key KEY --name NAME [--project KEY] [--description TEXT] [--due-date YYYY-MM-DD]\n\nCreate a roadmap feature. Example: htb feature create --key newsletter --name Newsletter --due-date 2026-09-30\n"
	case "ticket":
		return "Usage:\n  htb ticket create --title TITLE [options]\n  htb ticket list [options]\n  htb ticket show REF | comments REF | activity REF\n  htb ticket update --version N [options] REF\n  htb ticket comment REF TEXT\n  htb ticket publish REF | unpublish REF\n  htb ticket client-comments REF | client-comment REF TEXT\n  htb ticket claim REF | htb ticket release REF\n  htb ticket versions REF | restore --version N REF REVISION\n"
	case "ticket create":
		return "Usage: htb ticket create --title TITLE [--type user_story|technical_task|bug|incident] [--project KEY] [--parent REF] [--related REF] [--feature KEY] [--description TEXT] [--priority low|normal|high|urgent] [--label TAG]\n\nA technical task requires --parent STORY-REF. Repeat --label to add multiple labels.\n"
	case "ticket list":
		return "Usage: htb ticket list [--project KEY] [--feature KEY] [--status STATUS] [--priority PRIORITY] [--label LABEL] [--query TEXT] [--tree] [--json|--csv]\n\nFilter daily work by status, priority, label, or text in a title or description. Terminal output is used by default; use --json or --csv for scripts.\n"
	case "ticket show":
		return "Usage: htb ticket show REF\n\nShow a ticket and its current version.\n"
	case "ticket update":
		return "Usage: htb ticket update --version N [--title TITLE] [--description TEXT] [--status open|in_progress|review|blocked|done] [--priority low|normal|high|urgent] [--feature KEY] REF\n\nUse the version from " + `"htb ticket show REF"` + " to avoid overwriting a concurrent update. A user story status is calculated from its tasks.\n"
	case "ticket comment":
		return "Usage: htb ticket comment REF TEXT\n\nAdd a comment to a ticket.\n"
	case "ticket comments":
		return "Usage: htb ticket comments REF\n\nList a ticket conversation.\n"
	case "ticket publish", "ticket unpublish":
		return "Usage: htb " + command + " STORY-REF\n\nA project admin controls whether a user story is visible in the client portal. Technical tasks cannot be published.\n"
	case "ticket client-comments":
		return "Usage: htb ticket client-comments STORY-REF\n\nRead the separate client-visible conversation on a published user story.\n"
	case "ticket client-comment":
		return "Usage: htb ticket client-comment STORY-REF TEXT\n\nReply in the client-visible conversation. Internal ticket comments stay private.\n"
	case "ticket activity":
		return "Usage: htb ticket activity REF\n\nList the ticket audit activity.\n"
	case "ticket claim":
		return "Usage: htb ticket claim REF\n\nClaim a ticket for yourself.\n"
	case "ticket release":
		return "Usage: htb ticket release REF\n\nRelease your claim on a ticket.\n"
	case "ticket versions":
		return "Usage: htb ticket versions REF\n\nList a ticket's revisions.\n"
	case "ticket restore":
		return "Usage: htb ticket restore --version N REF REVISION\n\nRestore a revision when the ticket is still at version N.\n"
	case "request":
		return "Usage:\n  htb request list [--project KEY]\n  htb request show ID | comments ID\n  htb request comment ID TEXT\n  htb request status ID received|in_progress|needs_info|done|rejected\n  htb request link ID TICKET-REF\n\nClient requests are separate from internal work tickets.\n"
	case "invite":
		return "Usage:\n  htb invite create [--project KEY] [--role read|write|admin] [--expires-at RFC3339]\n  htb invite accept CODE\n  htb invite list [--project KEY]\n  htb invite revoke ID [--project KEY]\n"
	case "invite create":
		return "Usage: htb invite create [--project KEY] [--role read|write|admin] [--expires-at RFC3339]\n\nCreate an invitation for a project. It expires after 7 days by default.\n"
	case "invite accept":
		return "Usage: htb invite accept CODE\n\nAccept a project invitation.\n"
	case "invite list":
		return "Usage: htb invite list [--project KEY]\n\nList active invitations without exposing their codes.\n"
	case "invite revoke":
		return "Usage: htb invite revoke ID [--project KEY]\n\nRevoke an active invitation.\n"
	default:
		return fmt.Sprintf("Help is unavailable for \"htb %s\". Run \"htb help\".\n", command)
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
	issuer := fs.String("issuer", "", "sign-in issuer URL")
	clientID := fs.String("client-id", "", "device client ID")
	audience := fs.String("audience", "", "HTB audience ID")
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
		return errors.New("connection settings are unavailable; configure the server or provide --issuer, --client-id, and --audience")
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
			fmt.Fprintln(os.Stderr, "Opening sign-in page…")
		} else {
			fmt.Fprintln(os.Stderr, "Open this URL:", device.VerificationURIComplete)
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
	fmt.Printf("Connected: %s\nServer: %s\n", label, c.Server)
	if len(response.Projects) == 0 {
		fmt.Println("No accessible projects. Create one with 'htb project create' or join one with 'htb invite accept CODE'.")
		return nil
	}
	fmt.Println("Projects:")
	for _, project := range response.Projects {
		marker := " "
		if project.Key == c.CurrentProject {
			marker = "*"
		}
		fmt.Printf("%s %s — %s\n", marker, project.Key, project.Name)
	}
	if c.CurrentProject == "" {
		fmt.Printf("No current project. Run 'htb project use %s'.\n", response.Projects[0].Key)
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
	if len(args) > 0 && args[0] == "members" {
		return projectMembers(args[1:])
	}
	if len(args) > 0 && args[0] == "member" {
		return projectMember(args[1:])
	}
	if len(args) == 1 && args[0] == "status" {
		return projectStatus()
	}
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
				fmt.Println("Current project:", wanted)
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
			fmt.Println("No accessible projects.")
			return nil
		}
		c, err := load()
		if err != nil {
			return err
		}
		fmt.Println(styledHeading("Accessible projects:"))
		for _, project := range response.Projects {
			marker := " "
			if project.Key == c.CurrentProject {
				marker = "*"
			}
			if marker == "*" {
				marker = styledAccent(marker)
			}
			fmt.Printf("%s %s — %s%s\n", marker, styledReference(project.Key), project.Name, styledMuted(archivedLabel(project.Archived)))
		}
		return nil
	}
	if len(args) == 0 || args[0] != "create" {
		return errors.New("usage: htb project {list|status|use KEY|create|members|member}")
	}
	fs := flag.NewFlagSet("project create", flag.ContinueOnError)
	key := fs.String("key", "", "project key")
	name := fs.String("name", "", "name")
	description := fs.String("description", "", "description")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	normalizedKey, err := validateProjectCreation(*key, *name)
	if err != nil {
		return err
	}
	var created projectView
	if err := call("POST", "/api/v1/projects", map[string]string{"key": normalizedKey, "name": *name, "description": *description}, &created); err != nil {
		return err
	}
	c, err := load()
	if err != nil {
		return err
	}
	c.CurrentProject = normalizedKey
	if err := save(c); err != nil {
		return err
	}
	fmt.Printf("Project %s created: %s. It is now the current project.\n", created.Key, created.Name)
	return nil
}

func projectMembers(args []string) error {
	fs := flag.NewFlagSet("project members", flag.ContinueOnError)
	project := fs.String("project", "", "project")
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
	var response struct {
		Members []memberView `json:"members"`
	}
	if err := call("GET", "/api/v1/projects/"+*project+"/members", nil, &response); err != nil {
		return err
	}
	if len(response.Members) == 0 {
		fmt.Println("No project members.")
		return nil
	}
	fmt.Println(styledHeading("Project members"))
	for _, member := range response.Members {
		owner := ""
		if member.Owner {
			owner = " owner"
		}
		email := ""
		if member.Email != "" {
			email = " <" + member.Email + ">"
		}
		fmt.Printf("%d  %s%s — %s%s\n", member.UserID, member.Name, email, member.Role, styledMuted(owner))
	}
	return nil
}

func projectMember(args []string) error {
	if len(args) == 0 || (args[0] != "set-role" && args[0] != "remove") {
		return errors.New("usage: htb project member {set-role|remove} --user ID [--project KEY]")
	}
	fs := flag.NewFlagSet("project member "+args[0], flag.ContinueOnError)
	project := fs.String("project", "", "project")
	userID := fs.Int64("user", 0, "member ID")
	role := fs.String("role", "", "read|write|admin")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *userID < 1 {
		return errors.New("--user must be a member ID from htb project members")
	}
	if *project == "" {
		var err error
		*project, err = currentProject()
		if err != nil {
			return err
		}
	}
	path := fmt.Sprintf("/api/v1/projects/%s/members/%d", *project, *userID)
	if args[0] == "set-role" {
		if *role != "read" && *role != "write" && *role != "admin" {
			return errors.New("--role must be read, write, or admin")
		}
		if err := call("PATCH", path, map[string]string{"role": *role}, &struct{}{}); err != nil {
			return err
		}
		fmt.Printf("Member %d role set to %s.\n", *userID, *role)
		return nil
	}
	if err := call("DELETE", path, nil, &struct{}{}); err != nil {
		return err
	}
	fmt.Printf("Member %d removed.\n", *userID)
	return nil
}

func projectStatus() error {
	var response struct {
		Projects []projectStatusView `json:"projects"`
	}
	if err := call("GET", "/api/v1/projects/status", nil, &response); err != nil {
		return err
	}
	if len(response.Projects) == 0 {
		fmt.Println("No accessible projects.")
		return nil
	}
	c, err := load()
	if err != nil {
		return err
	}
	fmt.Println(styledHeading("Project status"))
	for _, project := range response.Projects {
		marker := " "
		if project.Key == c.CurrentProject {
			marker = styledAccent("*")
		}
		fmt.Printf("%s %s — %s%s\n", marker, styledReference(project.Key), project.Name, styledMuted(archivedLabel(project.Archived)))
		fmt.Printf("  User stories  %s\n", progressSummary(project.UserStories, "no user stories"))
		fmt.Printf("  Tickets       %s\n", progressSummary(project.Tickets, "no tickets"))
		fmt.Printf("  %s\n", statusBreakdown(project.Statuses))
	}
	return nil
}

func validateProjectCreation(key, name string) (string, error) {
	normalizedKey, err := domain.NormalizeProjectKey(key)
	if err != nil {
		return "", fmt.Errorf("%w. A project key identifies the project in commands and ticket references, for example HTB-1. Use 2 to 20 characters, starting with a letter; letters, digits, and underscores are allowed", err)
	}
	if strings.TrimSpace(name) == "" {
		return "", errors.New("project name is required")
	}
	return normalizedKey, nil
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
		due = " Due date: " + created.DueDate.Format("2006-01-02") + "."
	}
	fmt.Printf("Feature %s created: %s.%s\n", created.Key, created.Name, due)
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
	case "comments":
		return ticketComments(args[1:])
	case "publish":
		return ticketPublication(args[1:], true)
	case "unpublish":
		return ticketPublication(args[1:], false)
	case "client-comments":
		return ticketClientComments(args[1:])
	case "client-comment":
		return ticketClientComment(args[1:])
	case "activity":
		return ticketActivity(args[1:])
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
	fmt.Printf("Comment added to %s.\n", strings.ToUpper(args[0]))
	return nil
}
func ticketComments(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: htb ticket comments REF")
	}
	var response struct {
		Comments []commentView `json:"comments"`
	}
	if err := call("GET", "/api/v1/tickets/"+args[0]+"/comments", nil, &response); err != nil {
		return err
	}
	if len(response.Comments) == 0 {
		fmt.Println("No comments.")
		return nil
	}
	for _, comment := range response.Comments {
		fmt.Printf("%s — %s\n%s\n\n", comment.CreatedAt.Format(time.RFC3339), comment.Author, renderMarkdown(comment.Body))
	}
	return nil
}
func ticketActivity(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: htb ticket activity REF")
	}
	var response struct {
		Activity []activityView `json:"activity"`
	}
	if err := call("GET", "/api/v1/tickets/"+args[0]+"/activity", nil, &response); err != nil {
		return err
	}
	if len(response.Activity) == 0 {
		fmt.Println("No activity.")
		return nil
	}
	for _, item := range response.Activity {
		fmt.Printf("%s  %s — %s\n", item.CreatedAt.Format(time.RFC3339), item.Actor, item.Action)
	}
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
		fmt.Printf("Ticket %s released.\n", strings.ToUpper(args[0]))
		return nil
	}
	var ticket ticketView
	if err := call("POST", "/api/v1/tickets/"+args[0]+"/"+action, map[string]any{}, &ticket); err != nil {
		return err
	}
	fmt.Printf("Ticket %s claimed.\n", ticket.Ref)
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
	fmt.Printf("Ticket %s created — %s [%s].\n", ticket.Ref, ticket.Title, ticket.Status)
	return nil
}
func ticketList(args []string) error {
	fs := flag.NewFlagSet("ticket list", flag.ContinueOnError)
	project := fs.String("project", "", "project")
	tree := fs.Bool("tree", false, "include child tickets")
	feature := fs.String("feature", "", "feature key")
	status := fs.String("status", "", "open|in_progress|review|blocked|done")
	priority := fs.String("priority", "", "low|normal|high|urgent")
	label := fs.String("label", "", "label")
	search := fs.String("query", "", "text in title or description")
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
	if *status != "" {
		query.Set("status", *status)
	}
	if *priority != "" {
		query.Set("priority", *priority)
	}
	if *label != "" {
		query.Set("label", *label)
	}
	if *search != "" {
		query.Set("query", *search)
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
		fmt.Printf("No tickets in project %s.\n", strings.ToUpper(*project))
		return nil
	}
	heading := "Tickets — " + strings.ToUpper(*project)
	if *feature != "" {
		heading += " / feature " + *feature
	}
	if *status != "" {
		heading += " / " + *status
	}
	if *priority != "" {
		heading += " / " + *priority
	}
	if *label != "" {
		heading += " / label " + *label
	}
	if *search != "" {
		heading += " / “" + *search + "”"
	}
	fmt.Println(styledHeading(heading))
	fmt.Println("REF          STATUS         PROGRESS          TYPE              TITLE")
	for _, ticket := range response.Tickets {
		indent := ""
		if ticket.ParentRef != nil {
			indent = "↳ "
		}
		fmt.Printf("%s %s %s %-17s %s%s\n", paddedReference(ticket.Ref), paddedStatus(ticket.Status), paddedProgress(ticket), ticket.Type, indent, ticket.Title)
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
	fmt.Printf("Ticket %s updated (version %d).\n", ticket.Ref, ticket.Version)
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
	fmt.Printf("Ticket %s restored (version %d).\n", ticket.Ref, ticket.Version)
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
		fmt.Println("Invitation accepted. The project is now accessible.")
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
		if err := call("POST", "/api/v1/invitations", invitationCreateInput(*project, *role, *expires), &response); err != nil {
			return err
		}
		fmt.Printf("Invitation code created: %s\nShare it with: htb invite accept %s\n", response.Code, response.Code)
		return nil
	}
	if args[0] == "list" {
		fs := flag.NewFlagSet("invite list", flag.ContinueOnError)
		project := fs.String("project", "", "project")
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
			Invitations []invitationView `json:"invitations"`
		}
		if err := call("GET", "/api/v1/projects/"+*project+"/invitations", nil, &response); err != nil {
			return err
		}
		if len(response.Invitations) == 0 {
			fmt.Println("No active invitations.")
			return nil
		}
		fmt.Println(styledHeading("Active invitations"))
		for _, invitation := range response.Invitations {
			fmt.Printf("%d  %s — expires %s\n", invitation.ID, invitation.Role, invitation.ExpiresAt.Format(time.RFC3339))
		}
		return nil
	}
	if args[0] == "revoke" {
		fs := flag.NewFlagSet("invite revoke", flag.ContinueOnError)
		project := fs.String("project", "", "project")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 1 {
			return errors.New("usage: htb invite revoke ID [--project KEY]")
		}
		invitationID, err := strconv.ParseInt(fs.Arg(0), 10, 64)
		if err != nil || invitationID < 1 {
			return errors.New("invitation ID must be numeric")
		}
		if *project == "" {
			*project, err = currentProject()
			if err != nil {
				return err
			}
		}
		if err = call("DELETE", fmt.Sprintf("/api/v1/projects/%s/invitations/%d", *project, invitationID), nil, &struct{}{}); err != nil {
			return err
		}
		fmt.Printf("Invitation %d revoked.\n", invitationID)
		return nil
	}
	return errors.New("usage: htb invite {create|accept|list|revoke}")
}

func invitationCreateInput(project, role, expiresAt string) map[string]string {
	input := map[string]string{"project": project, "role": role}
	if expiresAt != "" {
		input["expires_at"] = expiresAt
	}
	return input
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
		return apiError(resp.Status, data)
	}
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if output != nil {
		return json.Unmarshal(data, output)
	}
	_, err = os.Stdout.Write(data)
	return err
}

func apiError(status string, data []byte) error {
	var response struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &response); err == nil && response.Error.Message != "" {
		return errors.New(response.Error.Message)
	}
	return fmt.Errorf("request failed: %s", status)
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
		return " (archived)"
	}
	return ""
}

func printTicket(ticket ticketView) {
	fmt.Printf("%s — %s\n", styledReference(ticket.Ref), ticket.Title)
	fmt.Printf("Project: %s | Type: %s | Status: %s | Priority: %s | Version: %d\n", ticket.Project, ticket.Type, styledStatus(ticket.Status), ticket.Priority, ticket.Version)
	if ticket.Type == "user_story" {
		fmt.Println("Progress:", ticketProgress(ticket))
	}
	if ticket.ParentRef != nil {
		fmt.Println("Parent ticket:", *ticket.ParentRef)
	}
	if ticket.RelatedRef != nil {
		fmt.Println("Related ticket:", *ticket.RelatedRef)
	}
	if ticket.FeatureKey != nil {
		fmt.Println("Feature:", *ticket.FeatureKey)
	}
	if len(ticket.Labels) > 0 {
		fmt.Println("Labels:", strings.Join(ticket.Labels, ", "))
	}
	if ticket.Description != "" {
		fmt.Printf("\n%s\n", renderMarkdown(ticket.Description))
	}
}

func ticketProgress(ticket ticketView) string {
	if ticket.Type != "user_story" {
		return "—"
	}
	if ticket.ChildCount == 0 {
		return "— no tasks"
	}
	percent := ticket.DoneChildren * 100 / ticket.ChildCount
	return fmt.Sprintf("%s %d/%d tasks (%d%%)", progressBar(ticket.DoneChildren, ticket.ChildCount), ticket.DoneChildren, ticket.ChildCount, percent)
}

func paddedProgress(ticket ticketView) string {
	progress := ticketProgress(ticket)
	plain := progress
	if ticket.Type == "user_story" && ticket.ChildCount > 0 {
		percent := ticket.DoneChildren * 100 / ticket.ChildCount
		plain = fmt.Sprintf("[██████████] %d/%d tasks (%d%%)", ticket.DoneChildren, ticket.ChildCount, percent)
	}
	return progress + strings.Repeat(" ", max(0, 31-utf8.RuneCountInString(plain)))
}

func progressSummary(progress progressView, empty string) string {
	if progress.Total == 0 {
		return styledMuted("— " + empty)
	}
	percent := progress.Done * 100 / progress.Total
	return fmt.Sprintf("%s %d/%d (%d%%)", progressBar(progress.Done, progress.Total), progress.Done, progress.Total, percent)
}

func progressBar(done, total int) string {
	if total == 0 {
		return "—"
	}
	filled := done * 10 / total
	return "[" + styledSuccess(strings.Repeat("█", filled)) + styledMuted(strings.Repeat("░", 10-filled)) + "]"
}

func statusBreakdown(counts statusCountsView) string {
	return strings.Join([]string{
		fmt.Sprintf("%s %d", styledStatus("open"), counts.Open),
		fmt.Sprintf("%s %d", styledStatus("in_progress"), counts.InProgress),
		fmt.Sprintf("%s %d", styledStatus("review"), counts.Review),
		fmt.Sprintf("%s %d", styledStatus("blocked"), counts.Blocked),
		fmt.Sprintf("%s %d", styledStatus("done"), counts.Done),
	}, " · ")
}

func styledStatus(status string) string {
	color := map[string]string{
		"open":        ansiMuted,
		"in_progress": ansiAccent,
		"review":      ansiWarning,
		"blocked":     ansiError,
		"done":        ansiSuccess,
	}[status]
	return styled(status, color)
}

const (
	ansiReset   = "\x1b[0m"
	ansiBold    = "\x1b[1m"
	ansiMuted   = "\x1b[90m"
	ansiAccent  = "\x1b[36m"
	ansiWarning = "\x1b[33m"
	ansiError   = "\x1b[31m"
	ansiSuccess = "\x1b[32m"
)

func styled(value, color string) string {
	if value == "" || color == "" || !colorsEnabled() {
		return value
	}
	return color + value + ansiReset
}

func styledHeading(value string) string   { return styled(value, ansiBold) }
func styledReference(value string) string { return styled(value, ansiBold) }
func styledMuted(value string) string     { return styled(value, ansiMuted) }
func styledAccent(value string) string    { return styled(value, ansiAccent) }
func styledError(value string) string     { return styled(value, ansiError) }
func styledSuccess(value string) string   { return styled(value, ansiSuccess) }

func paddedStatus(status string) string {
	return styledStatus(status) + strings.Repeat(" ", max(0, 14-len(status)))
}

func paddedReference(ref string) string {
	return styledReference(ref) + strings.Repeat(" ", max(0, 12-len(ref)))
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
