package api

import "net/http"

func (s *Server) routes() {
	s.router.HandleFunc("/health", s.Health)
	s.router.Handle("/maps", limitRequestBody(http.HandlerFunc(s.HandleMaps)))
	s.router.Handle("/maps/", limitRequestBody(http.HandlerFunc(s.HandleMapOperations)))
}
