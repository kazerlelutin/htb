package httpapi

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.v1.yaml
var openAPIV1 []byte

func (s *Server) openAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(openAPIV1)
}
