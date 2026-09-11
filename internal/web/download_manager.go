package web

import "net/http"

func (s *Server) downloadSeriesAPI(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "descarga automatica desactivada: los proveedores varian por serie", http.StatusGone)
}

func (s *Server) downloadStatusAPI(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "descarga automatica desactivada", http.StatusGone)
}
