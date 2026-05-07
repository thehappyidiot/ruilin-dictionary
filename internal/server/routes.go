package server

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/http/httputil"

	"github.com/thehappyidiot/ruilin-dictionary/internal/database"
	"github.com/thehappyidiot/ruilin-dictionary/internal/util"
)

const TYPE = "Content-Type"
const TYPE_HTML = "text/html; charset=utf-8"
const TYPE_PLAIN = "text/plain; charset=utf-8"
const TYPE_JSON = "text/json; charset=utf-8"

const INTERNAL_ERROR = "Something went wrong"

func (server *Server) RegisterRoutes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", server.getRoot)
	mux.HandleFunc("GET /api/health", server.getApiHealth)

	return server.middlewareHandler(mux)
}

func (server *Server) middlewareHandler(handler http.Handler) http.Handler {
	return server.middlewareLogger(handler)
}

func (server *Server) middlewareLogger(handler http.Handler) http.Handler {
	if !server.config.isDevelopment {
		return handler
	}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		res, err := httputil.DumpRequest(req, true)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Print(string(res))
		handler.ServeHTTP(w, req)
	})
}

func (server *Server) getRoot(w http.ResponseWriter, req *http.Request) {
	homepageTemplate, err := template.ParseFiles("./frontend/index.html")
	if err != nil {
		http.Error(w, INTERNAL_ERROR, http.StatusInternalServerError)
		return
	}
	http.ServeFile(w, req, "./frontend/index.html")
}

func (server *Server) getApiHealth(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set(TYPE, TYPE_PLAIN)
	io.WriteString(w, "OK")
}
