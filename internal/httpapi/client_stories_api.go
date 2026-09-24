package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/kazerlelutin/htb/internal/store"
)

func (s *Server) clientStoryAPI(w http.ResponseWriter, r *http.Request, tail string) {
	parts := strings.Split(tail, "/")
	if len(parts) != 2 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "not_found", "Route not found", nil)
		return
	}
	ref := parts[0]
	switch {
	case parts[1] == "publication" && (r.Method == http.MethodPut || r.Method == http.MethodDelete):
		published := r.Method == http.MethodPut
		if err := s.stories.SetClientStoryPublished(r.Context(), actor(r), ref, published); err != nil {
			s.writeClientStoryError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ref": strings.ToUpper(ref), "published": published})
	case parts[1] == "comments" && r.Method == http.MethodGet:
		comments, err := s.stories.ListClientStoryComments(r.Context(), actor(r), ref)
		if err != nil {
			s.writeClientStoryError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"comments": comments})
	case parts[1] == "comments" && r.Method == http.MethodPost:
		var in struct {
			Body string `json:"body"`
		}
		if !decode(w, r, &in) {
			return
		}
		comment, err := s.stories.AddClientStoryComment(r.Context(), actor(r), ref, in.Body)
		if err != nil {
			s.writeClientStoryError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, comment)
	default:
		writeError(w, http.StatusNotFound, "not_found", "Route not found", nil)
	}
}

func (s *Server) writeClientStoryError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "You do not have permission for this operation", nil)
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "User story not found", nil)
	case errors.Is(err, store.ErrInvalidClientRequest):
		writeError(w, http.StatusUnprocessableEntity, "invalid_comment", "Comment must contain 1 to 20000 characters", nil)
	default:
		s.log.Error("client story API", "error", err)
		writeError(w, http.StatusInternalServerError, "server_error", "Unable to process user story", nil)
	}
}
