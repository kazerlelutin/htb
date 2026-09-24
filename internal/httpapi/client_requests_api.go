package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/kazerlelutin/htb/internal/store"
)

func (s *Server) listInternalClientRequests(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	if project == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "project is required", nil)
		return
	}
	items, err := s.requests.ListClientRequests(r.Context(), actor(r), project)
	if err != nil {
		s.writeClientRequestError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"requests": items})
}

func (s *Server) getInternalClientRequest(w http.ResponseWriter, r *http.Request, path string) {
	comments := strings.HasSuffix(path, "/comments")
	if comments {
		path = strings.TrimSuffix(path, "/comments")
	}
	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request ID must be numeric", nil)
		return
	}
	if comments {
		items, err := s.requests.ListClientRequestComments(r.Context(), actor(r), id)
		if err != nil {
			s.writeClientRequestError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"comments": items})
		return
	}
	item, err := s.requests.GetClientRequest(r.Context(), actor(r), id)
	if err != nil {
		s.writeClientRequestError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) commentInternalClientRequest(w http.ResponseWriter, r *http.Request, path string) {
	if !strings.HasSuffix(path, "/comments") {
		writeError(w, http.StatusNotFound, "not_found", "Route not found", nil)
		return
	}
	id, err := strconv.ParseInt(strings.TrimSuffix(path, "/comments"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request ID must be numeric", nil)
		return
	}
	var in struct {
		Body string `json:"body"`
	}
	if !decode(w, r, &in) {
		return
	}
	comment, err := s.requests.AddClientRequestComment(r.Context(), actor(r), id, in.Body)
	if err != nil {
		s.writeClientRequestError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, comment)
}

func (s *Server) updateInternalClientRequest(w http.ResponseWriter, r *http.Request, rawID string) {
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request ID must be numeric", nil)
		return
	}
	var update store.ClientRequestUpdate
	if !decode(w, r, &update) {
		return
	}
	item, err := s.requests.UpdateClientRequest(r.Context(), actor(r), id, update)
	if err != nil {
		s.writeClientRequestError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) writeClientRequestError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "You do not have permission for this operation", nil)
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Request or ticket not found", nil)
	case errors.Is(err, store.ErrInvalidClientRequest):
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", err.Error(), nil)
	default:
		s.log.Error("client request API", "error", err)
		writeError(w, http.StatusInternalServerError, "server_error", "Unable to process client request", nil)
	}
}
