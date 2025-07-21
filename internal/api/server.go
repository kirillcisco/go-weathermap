package api

import (
	"fmt"
	"log"
	"net/http"

	"go-weathermap/internal/service"
	"go-weathermap/internal/utils"
)

const maxRequestBodySize = 1048576

type Server struct {
	mapService        *service.MapService
	dataSourceService *service.DataSourceService
	router            *http.ServeMux
}

func NewServer(mapService *service.MapService, dsService *service.DataSourceService) *Server {
	s := &Server{
		mapService:        mapService,
		dataSourceService: dsService,
		router:            http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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
