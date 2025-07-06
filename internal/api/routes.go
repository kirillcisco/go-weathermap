package api

import "net/http"

func (s *Server) routes() {
	s.router.HandleFunc("/health", s.Health)
	s.router.Handle("/maps", limitRequestBody(http.HandlerFunc(s.HandleMaps)))
	s.router.Handle("/maps/", limitRequestBody(http.HandlerFunc(s.HandleMapOperations)))
	s.router.HandleFunc("/icons", s.HandleIcons)
	s.router.HandleFunc("/icons/", s.HandleIconFile)
}
