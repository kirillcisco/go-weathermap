package api

import (
	"fmt"
	"log"
	"net/http"

	"go-weathermap/internal/service"
)

const maxRequestBodySize = 1048576

type Server struct {
	mapService *service.MapService
	router     *http.ServeMux
}

func NewServer(configDir string) *Server {
	s := &Server{
		mapService: service.NewMapService(configDir),
		router:     http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) Start(addr string) {
	fmt.Printf("Starting weathermap server on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, s))
}

func limitRequestBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
		next.ServeHTTP(w, r)
	})
}
