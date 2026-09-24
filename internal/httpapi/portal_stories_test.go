package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/kazerlelutin/htb/internal/store"
)

type clientStoryStub struct {
	stories   []store.ClientStory
	comments  []store.ClientStoryComment
	commented bool
	published bool
}

func (stub *clientStoryStub) ListClientStories(_ context.Context, _ store.Actor, project string) ([]store.ClientStory, error) {
	if project != "SITE" {
		return nil, store.ErrForbidden
	}
	return stub.stories, nil
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
	s.stories = &clientStoryStub{stories: []store.ClientStory{{Ref: "SITE-12", Project: "SITE", Title: "Exporter les données", Description: "# Besoin\n<script>secret()</script>", Status: "in_progress", ChildCount: 3, DoneChildren: 2}}}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, portalRequest(http.MethodGet, "/portal/projects/SITE", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("project dashboard: %d %s", w.Code, w.Body.String())
	}
	for _, want := range []string{"Exporter les données", "2 / 3 (66 %)", `href="/portal/stories/SITE-12"`, `value="2" max="3"`} {
		if !strings.Contains(w.Body.String(), want) {
			t.Fatalf("dashboard missing %q", want)
		}
	}
	for _, private := range []string{"Implement CSV endpoint", "priority", "assignee"} {
		if strings.Contains(w.Body.String(), private) {
			t.Fatalf("dashboard leaked %q", private)
		}
	}
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, portalRequest(http.MethodGet, "/portal/stories/SITE-12", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "<h2>Besoin</h2>") || strings.Contains(w.Body.String(), "<script>") {
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
	s.stories = &clientStoryStub{stories: []store.ClientStory{{Ref: "SITE-12", Project: "SITE", Title: "New story", Status: "open"}}}
	for _, path := range []string{"/portal/projects/SITE", "/portal/stories/SITE-12"} {
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, portalRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Aucune tâche technique liée") && !strings.Contains(w.Body.String(), "pas encore de tâche technique liée") {
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

func TestClientStoryCommentUsesPublicConversationAndCSRF(t *testing.T) {
	s, _, _ := portalTestServer()
	stories := &clientStoryStub{stories: []store.ClientStory{{Ref: "SITE-12", Project: "SITE", Title: "Exporter les données"}}}
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
