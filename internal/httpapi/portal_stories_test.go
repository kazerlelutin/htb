package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/kazerlelutin/htb/internal/store"
)

type clientStoryStub struct {
	stories           []store.ClientStory
	comments          []store.ClientStoryComment
	internalComments  []store.Comment
	commented         bool
	internalCommented bool
	published         bool
}

func (stub *clientStoryStub) ListClientStories(_ context.Context, _ store.Actor, project string) ([]store.ClientStory, error) {
	if project != "SITE" {
		return nil, store.ErrForbidden
	}
	return stub.stories, nil
}

func (stub *clientStoryStub) ListClientStoriesPage(_ context.Context, _ store.Actor, project string, page, perPage int) (store.ClientStoryPage, error) {
	if project != "SITE" {
		return store.ClientStoryPage{}, store.ErrForbidden
	}
	result := store.ClientStoryPage{Page: page, Total: len(stub.stories)}
	for _, story := range stub.stories {
		result.TaskCount += story.ChildCount
		result.TaskDone += story.DoneChildren
		if story.Status == "done" {
			result.StoryDone++
		}
	}
	result.TotalPages = (result.Total + perPage - 1) / perPage
	if result.TotalPages == 0 {
		result.TotalPages = 1
	}
	if result.Page < 1 {
		result.Page = 1
	}
	if result.Page > result.TotalPages {
		result.Page = result.TotalPages
	}
	start := (result.Page - 1) * perPage
	end := start + perPage
	if end > result.Total {
		end = result.Total
	}
	result.Stories = append(result.Stories, stub.stories[start:end]...)
	return result, nil
}

func (stub *clientStoryStub) GetClientStory(_ context.Context, _ store.Actor, ref string) (store.ClientStory, error) {
	for _, story := range stub.stories {
		if story.Ref == strings.ToUpper(ref) {
			return story, nil
		}
	}
	return store.ClientStory{}, store.ErrNotFound
}

func (stub *clientStoryStub) SetClientStoryPublished(_ context.Context, _ store.Actor, ref string, published bool) error {
	if ref != "SITE-12" {
		return store.ErrNotFound
	}
	stub.published = published
	return nil
}

func (stub *clientStoryStub) ListInternalStoryComments(ctx context.Context, actor store.Actor, ref string) ([]store.Comment, error) {
	if _, err := stub.GetClientStory(ctx, actor, ref); err != nil {
		return nil, err
	}
	return stub.internalComments, nil
}

func (stub *clientStoryStub) AddInternalStoryComment(ctx context.Context, actor store.Actor, ref, body string) (store.Comment, error) {
	if _, err := stub.GetClientStory(ctx, actor, ref); err != nil {
		return store.Comment{}, err
	}
	if strings.TrimSpace(body) == "" {
		return store.Comment{}, store.ErrInvalidClientRequest
	}
	stub.internalCommented = true
	return store.Comment{ID: 1, Body: body}, nil
}

func (stub *clientStoryStub) ListClientStoryComments(ctx context.Context, actor store.Actor, ref string) ([]store.ClientStoryComment, error) {
	if _, err := stub.GetClientStory(ctx, actor, ref); err != nil {
		return nil, err
	}
	return stub.comments, nil
}

func (stub *clientStoryStub) AddClientStoryComment(ctx context.Context, actor store.Actor, ref, body string) (store.ClientStoryComment, error) {
	if _, err := stub.GetClientStory(ctx, actor, ref); err != nil {
		return store.ClientStoryComment{}, err
	}
	if strings.TrimSpace(body) == "" {
		return store.ClientStoryComment{}, store.ErrInvalidClientRequest
	}
	stub.commented = true
	return store.ClientStoryComment{ID: 1, Body: body}, nil
}

func TestClientStoryDashboardShowsSafeProgress(t *testing.T) {
	s, _, _ := portalTestServer()
	s.stories = &clientStoryStub{stories: []store.ClientStory{{Ref: "SITE-12", Project: "SITE", Title: "Exporter les données", Description: "# Besoin\n<script>secret()</script>", Status: "in_progress", ChildCount: 3, DoneChildren: 2, Published: true}}}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, portalRequest(http.MethodGet, "/portal/projects/SITE", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("project dashboard: %d %s", w.Code, w.Body.String())
	}
	for _, want := range []string{"Exporter les données", "2 / 3 (66 %)", "Tâches terminées", `href="/portal/stories/SITE-12"`, `value="2" max="3"`, `<span class="cursor" aria-hidden="true">_</span>`} {
		if !strings.Contains(w.Body.String(), want) {
			t.Fatalf("dashboard missing %q", want)
		}
	}
	if strings.Contains(w.Body.String(), `<p class="eyebrow">SITE</p>`) || strings.Contains(w.Body.String(), "Tâches techniques") {
		t.Fatalf("dashboard contains redundant project key or overly technical task label: %s", w.Body.String())
	}
	for _, private := range []string{"Implement CSV endpoint", "priority", "assignee"} {
		if strings.Contains(w.Body.String(), private) {
			t.Fatalf("dashboard leaked %q", private)
		}
	}
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, portalRequest(http.MethodGet, "/portal/stories/SITE-12", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "<h2>Besoin</h2>") || !strings.Contains(w.Body.String(), "Tâches terminées") || !strings.Contains(w.Body.String(), `<span class="cursor" aria-hidden="true">_</span>`) || strings.Contains(w.Body.String(), "<script>") {
		t.Fatalf("unsafe story detail: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, portalRequest(http.MethodGet, "/portal/stories/SITE-13", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("unpublished story returned %d", w.Code)
	}
}

func TestClientStoryWithoutTasksHasNoMisleadingPercentage(t *testing.T) {
	s, _, _ := portalTestServer()
	s.stories = &clientStoryStub{stories: []store.ClientStory{{Ref: "SITE-12", Project: "SITE", Title: "New story", Status: "open", Published: true}}}
	for _, path := range []string{"/portal/projects/SITE", "/portal/stories/SITE-12"} {
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, portalRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Aucune tâche liée") && !strings.Contains(w.Body.String(), "pas encore de tâche liée") {
			t.Fatalf("%s: missing no-task explanation: %d %s", path, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "Tâches terminées : 0 / 0") || strings.Contains(w.Body.String(), `id="story-progress"`) {
			t.Fatalf("%s: misleading progress: %s", path, w.Body.String())
		}
	}
	if got := progressPercent(2, 3); got != 66 {
		t.Fatalf("two completed tasks out of three: got %d%%", got)
	}
}

func TestClientStoryDashboardPaginatesStories(t *testing.T) {
	s, _, _ := portalTestServer()
	stories := make([]store.ClientStory, 21)
	for i := range stories {
		stories[i] = store.ClientStory{Ref: "SITE-" + strconv.Itoa(i+1), Project: "SITE", Title: "Story " + strconv.Itoa(i+1), Published: true}
	}
	s.stories = &clientStoryStub{stories: stories}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, portalRequest(http.MethodGet, "/portal/projects/SITE?page=2", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("page 2: %d %s", w.Code, w.Body.String())
	}
	for _, want := range []string{"Story 21", "Page 2 sur 2", `href="/portal/projects/SITE?page=1"`, `rel="prev"`} {
		if !strings.Contains(w.Body.String(), want) {
			t.Fatalf("page 2 missing %q: %s", want, w.Body.String())
		}
	}
	if strings.Contains(w.Body.String(), "Story 1") {
		t.Fatalf("page 2 contains a story from page 1: %s", w.Body.String())
	}
}

func TestAdministratorCanAddInternalCommentToDraftStory(t *testing.T) {
	s, _, _ := portalTestServer()
	stories := &clientStoryStub{stories: []store.ClientStory{{Ref: "SITE-12", Project: "SITE", Title: "Projet privé", Status: "open"}}}
	s.stories = stories
	for _, path := range []string{"/portal/projects/SITE", "/portal/stories/SITE-12"} {
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, portalRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Brouillon") {
			t.Fatalf("%s: draft is missing: %d %s", path, w.Code, w.Body.String())
		}
		if path == "/portal/stories/SITE-12" && (!strings.Contains(w.Body.String(), "htb ticket publish SITE-12") || !strings.Contains(w.Body.String(), "Ajouter un commentaire (Markdown accepté)") || strings.Contains(w.Body.String(), "commentaire interne") || !strings.Contains(w.Body.String(), `action="/portal/stories/SITE-12/ticket-comments"`)) {
			t.Fatalf("draft detail has wrong actions: %s", w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, portalRequest(http.MethodPost, "/portal/stories/SITE-12/ticket-comments", url.Values{"body": {"Note interne"}}))
	if w.Code != http.StatusForbidden || stories.internalCommented {
		t.Fatalf("internal comment without CSRF: %d", w.Code)
	}
	csrf := s.portalCSRF(portalRequest(http.MethodGet, "/portal", nil).WithContext(context.WithValue(context.Background(), browserSessionTokenKey{}, "valid")))
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, portalRequest(http.MethodPost, "/portal/stories/SITE-12/ticket-comments", url.Values{"csrf": {csrf}, "body": {"Note interne"}}))
	if w.Code != http.StatusSeeOther || !stories.internalCommented {
		t.Fatalf("valid internal story comment: %d %s", w.Code, w.Body.String())
	}
}

func TestClientStoryCommentUsesPublicConversationAndCSRF(t *testing.T) {
	s, _, _ := portalTestServer()
	stories := &clientStoryStub{stories: []store.ClientStory{{Ref: "SITE-12", Project: "SITE", Title: "Exporter les données", Published: true}}}
	s.stories = stories
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, portalRequest(http.MethodPost, "/portal/stories/SITE-12/comments", url.Values{"body": {"Question"}}))
	if w.Code != http.StatusForbidden || stories.commented {
		t.Fatalf("comment without CSRF: %d", w.Code)
	}
	csrf := s.portalCSRF(portalRequest(http.MethodGet, "/portal", nil).WithContext(context.WithValue(context.Background(), browserSessionTokenKey{}, "valid")))
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, portalRequest(http.MethodPost, "/portal/stories/SITE-12/comments", url.Values{"csrf": {csrf}, "body": {"Question"}}))
	if w.Code != http.StatusSeeOther || !stories.commented {
		t.Fatalf("valid story comment: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, portalRequest(http.MethodPost, "/portal/stories/SITE-13/comments", url.Values{"csrf": {csrf}, "body": {"Question"}}))
	if w.Code != http.StatusNotFound {
		t.Fatalf("comment on hidden story returned %d", w.Code)
	}
}

func TestClientStoryAPIKeepsPublicationAndCommentsSeparate(t *testing.T) {
	s, _, _ := portalTestServer()
	stories := &clientStoryStub{stories: []store.ClientStory{{Ref: "SITE-12", Project: "SITE", Title: "Exporter"}}}
	s.stories = stories
	for _, test := range []struct {
		method, tail string
		want         int
	}{
		{http.MethodPut, "SITE-12/publication", http.StatusOK},
		{http.MethodDelete, "SITE-12/publication", http.StatusOK},
		{http.MethodGet, "SITE-12/comments", http.StatusOK},
		{http.MethodPut, "SITE-12/comments", http.StatusNotFound},
	} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(test.method, "/api/v1/client-stories/"+test.tail, nil).WithContext(context.WithValue(context.Background(), actorKey{}, store.Actor{CredentialID: 1}))
		s.clientStoryAPI(w, r, test.tail)
		if w.Code != test.want {
			t.Errorf("%s %s: got %d, want %d", test.method, test.tail, w.Code, test.want)
		}
	}
}
