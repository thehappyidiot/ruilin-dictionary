package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"strconv"

	"github.com/thehappyidiot/ruilin-dictionary/internal/database"
)

const TYPE = "Content-Type"
const TYPE_HTML = "text/html; charset=utf-8"
const TYPE_PLAIN = "text/plain; charset=utf-8"

const INTERNAL_ERROR = "Something went wrong"

func (server *Server) RegisterRoutes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", server.getRoot)
	mux.HandleFunc("GET /random", server.getRandomWord)
	mux.HandleFunc("GET /word/{id}", server.getWord)
	mux.HandleFunc("GET /search", server.getSearch)
	mux.HandleFunc("GET /api/health", server.getApiHealth)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./frontend"))))

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

type WordPageData struct {
	Word       database.Word
	Confusions []database.Word
}

type SearchPageData struct {
	Query   string
	Results []database.Word
}

func renderTemplate(w http.ResponseWriter, path string, data any) {
	tmpl, err := template.ParseFiles(path)
	if err != nil {
		log.Printf("template parse error (%s): %v", path, err)
		http.Error(w, INTERNAL_ERROR, http.StatusInternalServerError)
		return
	}
	w.Header().Set(TYPE, TYPE_HTML)
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("template execute error (%s): %v", path, err)
	}
}

func (server *Server) getRoot(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		renderTemplate(w, "./frontend/404.html", nil)
		return
	}
	http.Redirect(w, req, "/random", http.StatusTemporaryRedirect)
}

func (server *Server) getRandomWord(w http.ResponseWriter, req *http.Request) {
	word, err := server.dbQueries.GetRandomWord(context.Background())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			renderTemplate(w, "./frontend/404.html", nil)
			return
		}
		log.Printf("getRandomWord db error: %v", err)
		http.Error(w, INTERNAL_ERROR, http.StatusInternalServerError)
		return
	}
	http.Redirect(w, req, fmt.Sprintf("/word/%d", word.ID), http.StatusTemporaryRedirect)
}

func (server *Server) getWord(w http.ResponseWriter, req *http.Request) {
	idStr := req.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		renderTemplate(w, "./frontend/404.html", nil)
		return
	}

	word, err := server.dbQueries.GetWordByID(context.Background(), int32(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			renderTemplate(w, "./frontend/404.html", nil)
			return
		}
		log.Printf("getWord db error: %v", err)
		http.Error(w, INTERNAL_ERROR, http.StatusInternalServerError)
		return
	}

	confusions, err := server.dbQueries.GetWordConfusions(context.Background(), word.ID)
	if err != nil {
		log.Printf("getWord confusions db error: %v", err)
		http.Error(w, INTERNAL_ERROR, http.StatusInternalServerError)
		return
	}

	renderTemplate(w, "./frontend/index.html", WordPageData{
		Word:       word,
		Confusions: confusions,
	})
}

func (server *Server) getSearch(w http.ResponseWriter, req *http.Request) {
	query := req.URL.Query().Get("q")

	var results []database.Word
	if query != "" {
		results, _ = server.dbQueries.SearchWords(context.Background(), sql.NullString{String: query, Valid: true})
	}

	renderTemplate(w, "./frontend/search.html", SearchPageData{
		Query:   query,
		Results: results,
	})
}

func (server *Server) getApiHealth(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set(TYPE, TYPE_PLAIN)
	io.WriteString(w, "OK")
}
