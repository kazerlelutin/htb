package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadUsesHostedServerByDefault(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	c, err := load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Server != defaultServer {
		t.Fatalf("default server = %q, want %q", c.Server, defaultServer)
	}
}

func TestLoadPreservesConfiguredServer(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := save(config{Server: "https://tickets.example.org"}); err != nil {
		t.Fatal(err)
	}
	c, err := load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Server != "https://tickets.example.org" {
		t.Fatalf("configured server = %q", c.Server)
	}
}

func TestDeviceScopesRequestHTBAudienceAndRoles(t *testing.T) {
	scopes := deviceScopes("123456")
	for _, expected := range []string{
		"urn:zitadel:iam:org:project:id:123456:aud",
		"urn:zitadel:iam:org:projects:roles",
	} {
		if !strings.Contains(scopes, expected) {
			t.Fatalf("missing %q in %q", expected, scopes)
		}
	}
}

func TestArchivedLabel(t *testing.T) {
	if got := archivedLabel(true); got != " (archived)" {
		t.Fatalf("got %q", got)
	}
	if got := archivedLabel(false); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestHelpRecognizesShortAndLongFlags(t *testing.T) {
	if !isHelpFlag("-h") || !isHelpFlag("--help") || isHelpFlag("help") {
		t.Fatal("help flags are not recognized correctly")
	}
}

func TestTicketViewDecodesRelationshipAndPublicationFields(t *testing.T) {
	var ticket ticketView
	if err := json.Unmarshal([]byte(`{"ref":"SITE-4","parent_ref":"SITE-3","feature_key":"newsletter","published":true,"archived":true}`), &ticket); err != nil {
		t.Fatal(err)
	}
	if ticket.ParentRef == nil || *ticket.ParentRef != "SITE-3" || ticket.FeatureKey == nil || *ticket.FeatureKey != "newsletter" {
		t.Fatalf("unexpected ticket view: %#v", ticket)
	}
	if !ticket.Published {
		t.Fatal("published state was not decoded")
	}
	if !ticket.Archived {
		t.Fatal("archive state was not decoded")
	}
}

func TestTicketProgress(t *testing.T) {
	if got := ticketProgress(ticketView{Type: "user_story", ChildCount: 3, DoneChildren: 2}); got != "[██████░░░░] 2/3 tasks (66%)" {
		t.Fatalf("got %q", got)
	}
	if got := ticketProgress(ticketView{Type: "user_story"}); got != "— no tasks" {
		t.Fatalf("got %q", got)
	}
	if got := ticketProgress(ticketView{Type: "bug"}); got != "—" {
		t.Fatalf("got %q", got)
	}
}

func TestProgressBarAndColorsRespectConfiguration(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("HTB_COLOR", "always")
	if got := progressBar(1, 2); got != "[█████░░░░░]" {
		t.Fatalf("uncolored bar = %q", got)
	}
	if got := progressSummary(progressView{Done: 0, Total: 0}, "no tickets"); got != "— no tickets" {
		t.Fatalf("empty progress = %q", got)
	}
	t.Setenv("NO_COLOR", "")
	t.Setenv("HTB_COLOR", "always")
	if got := progressBar(1, 2); !strings.Contains(got, ansiSuccess) || !strings.Contains(got, ansiMuted) {
		t.Fatalf("colored bar = %q", got)
	}
	if got := styledStatus("blocked"); !strings.Contains(got, ansiError) {
		t.Fatalf("blocked status = %q", got)
	}
}

func TestRenderMarkdownMakesTicketTextReadableWithoutColors(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got := renderMarkdown("# Demande\n\n- [ ] Vérifier **le flux**\n- [x] Lire [le guide](https://example.test/guide)\n\n> _Réponse attendue_\n\n```go\nfmt.Println(\"ok\")\n```")
	want := "Demande\n\n☐ Vérifier le flux\n☑ Lire le guide <https://example.test/guide>\n\n│ Réponse attendue\n\n  fmt.Println(\"ok\")"
	if got != want {
		t.Fatalf("rendered Markdown = %q, want %q", got, want)
	}
}

func TestRenderMarkdownDoesNotEmitAuthorSuppliedTerminalControls(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got := renderMarkdown("# Bonjour\x1b[2J\nTexte\r masqué\u009b2Jvisible")
	if strings.ContainsAny(got, "\x1b\u009b\r") {
		t.Fatalf("rendered Markdown contains a terminal control: %q", got)
	}
	if got != "Bonjour\nTexte masquévisible" {
		t.Fatalf("unexpected sanitized Markdown: %q", got)
	}
}

func TestProjectStatusViewDecodesAggregate(t *testing.T) {
	var response struct {
		Projects []projectStatusView `json:"projects"`
	}
	if err := json.Unmarshal([]byte(`{"projects":[{"key":"SITE","name":"Site","role":"read","user_stories":{"total":3,"done":2},"tickets":{"total":5,"done":3},"statuses":{"open":1,"in_progress":1,"review":0,"blocked":0,"done":3}}]}`), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Projects) != 1 || response.Projects[0].Key != "SITE" || response.Projects[0].Role != "read" || response.Projects[0].UserStories.Done != 2 || response.Projects[0].Statuses.InProgress != 1 {
		t.Fatalf("unexpected project status: %#v", response.Projects)
	}
}

func TestNormalizeNamespaceFilter(t *testing.T) {
	for input, want := range map[string]string{
		"alice":   "ALICE",
		" Alice ": "ALICE",
		"":        "",
	} {
		if got := normalizeNamespaceFilter(input); got != want {
			t.Errorf("normalizeNamespaceFilter(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestProjectCreationValidationExplainsAndNormalizesKey(t *testing.T) {
	key, err := validateProjectCreation(" htb ", "HTB")
	if err != nil || key != "HTB" {
		t.Fatalf("got key=%q err=%v", key, err)
	}
	for _, input := range []string{"", "htb-key"} {
		if _, err := validateProjectCreation(input, "HTB"); err == nil || !strings.Contains(err.Error(), "project key") || !strings.Contains(err.Error(), "HTB-1") {
			t.Fatalf("input %q returned %v", input, err)
		}
	}
	if _, err := validateProjectCreation("HTB", " "); err == nil || err.Error() != "project name is required" {
		t.Fatalf("unexpected name validation error: %v", err)
	}
}

func TestHelpIsDetailedForEveryCommand(t *testing.T) {
	commands := [][]string{
		{"version"}, {"update"}, {"config", "set-server"}, {"auth", "login"}, {"auth", "status"}, {"namespace"}, {"namespace", "claim"}, {"namespace", "list"},
		{"project", "list"}, {"project", "status"}, {"project", "use"}, {"project", "create"}, {"project", "members"}, {"project", "member"}, {"feature", "create"},
		{"ticket", "create"}, {"ticket", "list"}, {"ticket", "show"}, {"ticket", "update"}, {"ticket", "comment"}, {"ticket", "claim"}, {"ticket", "release"}, {"ticket", "versions"}, {"ticket", "restore"}, {"ticket", "archive"}, {"ticket", "unarchive"}, {"ticket", "delete"},
		{"invite", "create"}, {"invite", "accept"}, {"invite", "list"}, {"invite", "revoke"},
	}
	for _, command := range commands {
		help := helpText(command)
		if !strings.Contains(help, "Usage:") || strings.Contains(help, "Help is unavailable") {
			t.Fatalf("command %q has incomplete help: %s", command, help)
		}
	}
	if help := helpText([]string{"project", "create"}); !strings.Contains(help, "normalized to uppercase") || !strings.Contains(help, "HTB-1") {
		t.Fatalf("project help lacks key explanation: %s", help)
	}
}

func TestUpdateAssetMatchesReleasePlatforms(t *testing.T) {
	tests := []struct {
		goos, goarch, archive, binary string
		zip                           bool
	}{
		{"linux", "amd64", "htb_linux_amd64.tar.gz", "htb", false},
		{"darwin", "amd64", "htb_darwin_amd64.tar.gz", "htb", false},
		{"darwin", "arm64", "htb_darwin_arm64.tar.gz", "htb", false},
		{"windows", "amd64", "htb_windows_amd64.zip", "htb.exe", true},
	}
	for _, test := range tests {
		asset, err := updateAssetForPlatform(test.goos, test.goarch)
		if err != nil {
			t.Fatalf("%s/%s: %v", test.goos, test.goarch, err)
		}
		if asset.archive != test.archive || asset.binary != test.binary || asset.zip != test.zip {
			t.Fatalf("%s/%s: %#v", test.goos, test.goarch, asset)
		}
	}
	if _, err := updateAssetForPlatform("linux", "arm64"); err == nil || !strings.Contains(err.Error(), "linux/arm64") {
		t.Fatalf("unsupported platform error = %v", err)
	}
}

func TestReleaseChecksumAcceptsStandardChecksumFormats(t *testing.T) {
	hash := strings.Repeat("a", 64)
	checksums := []byte(hash + "  htb_linux_amd64.tar.gz\n" + hash + " *htb_windows_amd64.zip\n")
	for _, archive := range []string{"htb_linux_amd64.tar.gz", "htb_windows_amd64.zip"} {
		got, err := releaseChecksum(checksums, archive)
		if err != nil || got != hash {
			t.Fatalf("checksum for %s = %q, %v", archive, got, err)
		}
	}
	if _, err := releaseChecksum(checksums, "htb_darwin_arm64.tar.gz"); err == nil {
		t.Fatal("missing checksum was accepted")
	}
}

func TestInstallLatestUpdateVerifiesAndReplacesInstalledCLIBinary(t *testing.T) {
	var archive bytes.Buffer
	compressed := gzip.NewWriter(&archive)
	writer := tar.NewWriter(compressed)
	payload := []byte("new HTB CLI")
	if err := writer.WriteHeader(&tar.Header{Name: "htb", Mode: 0755, Size: int64(len(payload))}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	archiveBytes := archive.Bytes()
	checksum := sha256.Sum256(archiveBytes)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/checksums.txt":
			_, _ = response.Write([]byte(fmt.Sprintf("%x  htb_linux_amd64.tar.gz\n", checksum)))
		case "/htb_linux_amd64.tar.gz":
			_, _ = response.Write(archiveBytes)
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	previousURL := releaseDownloadURL
	releaseDownloadURL = server.URL
	t.Cleanup(func() { releaseDownloadURL = previousURL })
	executable := filepath.Join(t.TempDir(), "htb")
	if err := os.WriteFile(executable, []byte("old HTB CLI"), 0755); err != nil {
		t.Fatal(err)
	}

	deferred, err := installLatestUpdate("linux", "amd64", executable, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if deferred {
		t.Fatal("Linux update was deferred")
	}
	got, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("updated executable = %q, want %q", got, payload)
	}
}

func TestLatestReleaseNotesFetchesGitHubMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.Header.Get("Accept") != "application/vnd.github+json" || request.Header.Get("User-Agent") != "htb-cli-updater" {
			t.Fatalf("unexpected release-notes request: %s %q %q", request.Method, request.Header.Get("Accept"), request.Header.Get("User-Agent"))
		}
		_, _ = response.Write([]byte(`{"tag_name":"v1.2.3","body":"## Added\n- Export tickets","html_url":"https://github.example/releases/v1.2.3"}`))
	}))
	defer server.Close()

	previousURL := releaseAPIURL
	releaseAPIURL = server.URL
	t.Cleanup(func() { releaseAPIURL = previousURL })

	notes, err := latestReleaseNotes(server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if notes.TagName != "v1.2.3" || notes.HTMLURL != "https://github.example/releases/v1.2.3" || !strings.Contains(notes.Body, "Export tickets") {
		t.Fatalf("release notes = %#v", notes)
	}
}

func TestLatestReleaseNotesRejectsIncompleteMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, _ = response.Write([]byte(`{"tag_name":"v1.2.3"}`))
	}))
	defer server.Close()

	previousURL := releaseAPIURL
	releaseAPIURL = server.URL
	t.Cleanup(func() { releaseAPIURL = previousURL })

	if _, err := latestReleaseNotes(server.Client()); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("incomplete release notes error = %v", err)
	}
}

func TestFormatReleaseNotesRendersSafely(t *testing.T) {
	output := formatReleaseNotes(releaseNotes{TagName: "v1.2.3\x1b]8;;https://evil.example\x1b\\", Body: "## Added\n- **Export** tickets\n\x1b]8;;https://evil.example\x1b\\", HTMLURL: "https://github.example/releases/v1.2.3\x1b]8;;https://evil.example\x1b\\"})
	for _, text := range []string{"What's new in v1.2.3", "Added", "Export", "Full release notes: https://github.example/releases/v1.2.3"} {
		if !strings.Contains(output, text) {
			t.Fatalf("release-notes output is missing %q: %q", text, output)
		}
	}
	if strings.Contains(output, "\x1b") || strings.Contains(output, "evil.example") {
		t.Fatalf("release-notes output contains terminal control content: %q", output)
	}
}

func TestTicketListHelpExplainsDailyWorkFilters(t *testing.T) {
	help := helpText([]string{"ticket", "list"})
	for _, flag := range []string{"--project KEY | --all-projects", "--status STATUS", "--priority PRIORITY", "--label LABEL", "--query TEXT", "--archived", "--json", "--csv"} {
		if !strings.Contains(help, flag) {
			t.Fatalf("ticket list help is missing %q: %s", flag, help)
		}
	}
}

func TestTicketListRejectsProjectAndAllProjectsTogether(t *testing.T) {
	err := ticketList([]string{"--project", "SITE", "--all-projects"})
	if err == nil || err.Error() != "--project and --all-projects cannot be used together" {
		t.Fatalf("unexpected conflicting project options error: %v", err)
	}
}

func TestAPIErrorUsesStructuredMessage(t *testing.T) {
	err := apiError("422 Unprocessable Entity", []byte(`{"error":{"message":"project key is required"}}`))
	if err.Error() != "project key is required" {
		t.Fatalf("got %q", err)
	}
	if err := apiError("500 Internal Server Error", []byte("not json")); err.Error() != "request failed: 500 Internal Server Error" {
		t.Fatalf("got %q", err)
	}
}

func TestInvitationCreateInputUsesOptionalExpiration(t *testing.T) {
	if got, want := invitationCreateInput("ANBY", "read", ""), map[string]string{"project": "ANBY", "role": "read"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("default invitation input = %#v, want %#v", got, want)
	}
	if got := invitationCreateInput("ANBY", "read", "2026-10-02T12:00:00Z"); got["expires_at"] != "2026-10-02T12:00:00Z" {
		t.Fatalf("explicit invitation expiration missing from %#v", got)
	}
}

func TestNamespaceInvitationInputUsesOptionalExpiration(t *testing.T) {
	if got, want := namespaceInvitationCreateInput("MO5", "read", ""), map[string]string{"namespace": "MO5", "role": "read"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("default namespace invitation input = %#v, want %#v", got, want)
	}
	if got := namespaceInvitationCreateInput("MO5", "read", "2026-10-02T12:00:00Z"); got["expires_at"] != "2026-10-02T12:00:00Z" {
		t.Fatalf("explicit namespace invitation expiration missing from %#v", got)
	}
}

func TestInviteRejectsProjectAndNamespaceTogether(t *testing.T) {
	for _, args := range [][]string{
		{"create", "--project", "SITE", "--namespace", "MO5"},
		{"list", "--project", "SITE", "--namespace", "MO5"},
		{"revoke", "1", "--project", "SITE", "--namespace", "MO5"},
	} {
		if err := inviteCommand(args); err == nil || err.Error() != "--project and --namespace cannot be used together" {
			t.Fatalf("args %v returned %v", args, err)
		}
	}
}
