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
	"time"

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
	mux.HandleFunc("GET /admin", server.getAdmin)
	mux.Handle("POST /admin/login", server.requireCSRF(http.HandlerFunc(server.postAdminLogin)))
	mux.Handle("POST /admin/logout", server.requireCSRF(server.requireAdmin(http.HandlerFunc(server.postAdminLogout))))
	mux.Handle("GET /admin/word/new", server.requireAdmin(http.HandlerFunc(server.getAdminNewWord)))
	mux.Handle("POST /admin/word/new", server.requireCSRF(server.requireAdmin(http.HandlerFunc(server.postAdminNewWord))))
	mux.Handle("GET /admin/word/{id}/edit", server.requireAdmin(http.HandlerFunc(server.getAdminEditWord)))
	mux.Handle("POST /admin/word/{id}/edit", server.requireCSRF(server.requireAdmin(http.HandlerFunc(server.postAdminEditWord))))
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
	Confusions []string
	IsAdmin    bool
}

type SearchPageData struct {
	Query   string
	Results []database.Word
	IsAdmin bool
}

type AdminLoginPageData struct {
	Error     string
	CSRFToken string
}

type WordFormValues struct {
	Word            string
	Type            string
	Meaning         string
	Sentence        string
	Origin          string
	ConfusedWithRaw string
	ConfusedWith    []string
}

type AdminWordFormPageData struct {
	Title        string
	SubmitAction string
	SubmitLabel  string
	Error        string
	CSRFToken    string
	Form         WordFormValues
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

	renderTemplate(w, "./frontend/index.html", WordPageData{
		Word:       word,
		Confusions: word.ConfusedWith,
		IsAdmin:    server.isAdmin(req),
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
		IsAdmin: server.isAdmin(req),
	})
}

func (server *Server) getAdmin(w http.ResponseWriter, req *http.Request) {
	if server.isAdmin(req) {
		http.Redirect(w, req, "/admin/word/new", http.StatusSeeOther)
		return
	}

	renderTemplate(w, "./frontend/admin_login.html", AdminLoginPageData{
		CSRFToken: server.ensureCSRFToken(w, req),
	})
}

func (server *Server) postAdminLogin(w http.ResponseWriter, req *http.Request) {
	password := req.FormValue("password")
	clientIP := getClientIP(req)
	if backoff := server.adminRateLimiter.Backoff(clientIP); backoff > 0 {
		time.Sleep(backoff)
	}
	if !server.adminRateLimiter.Allow(clientIP) {
		http.Error(w, "Too many login attempts, please try again later", http.StatusTooManyRequests)
		return
	}

	if !server.verifyAdminPassword(password) {
		server.adminRateLimiter.RecordFailure(clientIP)
		log.Printf("admin login failure from %s", clientIP)
		w.WriteHeader(http.StatusUnauthorized)
		renderTemplate(w, "./frontend/admin_login.html", AdminLoginPageData{
			Error:     "Invalid credentials",
			CSRFToken: server.ensureCSRFToken(w, req),
		})
		return
	}

	server.adminRateLimiter.RecordSuccess(clientIP)
	if !server.setAdmin(req, w, true) {
		return
	}
	http.Redirect(w, req, "/admin/word/new", http.StatusSeeOther)
}

func (server *Server) postAdminLogout(w http.ResponseWriter, req *http.Request) {
	if !server.setAdmin(req, w, false) {
		return
	}
	http.Redirect(w, req, "/admin", http.StatusSeeOther)
}

func (server *Server) getAdminNewWord(w http.ResponseWriter, req *http.Request) {
	renderTemplate(w, "./frontend/admin_word_form.html", AdminWordFormPageData{
		Title:        "Add a New Word",
		SubmitAction: "/admin/word/new",
		SubmitLabel:  "Save Word",
		CSRFToken:    server.ensureCSRFToken(w, req),
	})
}

func (server *Server) postAdminNewWord(w http.ResponseWriter, req *http.Request) {
	form, formErr := parseAndValidateWordForm(req)
	if formErr != "" {
		w.WriteHeader(http.StatusBadRequest)
		renderTemplate(w, "./frontend/admin_word_form.html", AdminWordFormPageData{
			Title:        "Add a New Word",
			SubmitAction: "/admin/word/new",
			SubmitLabel:  "Save Word",
			Error:        formErr,
			CSRFToken:    server.ensureCSRFToken(w, req),
			Form:         form,
		})
		return
	}

	word, err := server.dbQueries.CreateWord(context.Background(), database.CreateWordParams{
		Word:         form.Word,
		Type:         form.Type,
		Meaning:      form.Meaning,
		Sentence:     form.Sentence,
		Origin:       form.Origin,
		ConfusedWith: form.ConfusedWith,
	})
	if err != nil {
		log.Printf("create word db error: %v", err)
		http.Error(w, INTERNAL_ERROR, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, req, fmt.Sprintf("/word/%d", word.ID), http.StatusSeeOther)
}

func (server *Server) getAdminEditWord(w http.ResponseWriter, req *http.Request) {
	id, err := strconv.Atoi(req.PathValue("id"))
	if err != nil {
		renderTemplate(w, "./frontend/404.html", nil)
		return
	}

	word, err := server.dbQueries.GetWordByID(context.Background(), int32(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			renderTemplate(w, "./frontend/404.html", nil)
			return
		}
		log.Printf("getAdminEditWord db error: %v", err)
		http.Error(w, INTERNAL_ERROR, http.StatusInternalServerError)
		return
	}

	renderTemplate(w, "./frontend/admin_word_form.html", AdminWordFormPageData{
		Title:        "Edit Word",
		SubmitAction: fmt.Sprintf("/admin/word/%d/edit", id),
		SubmitLabel:  "Update Word",
		CSRFToken:    server.ensureCSRFToken(w, req),
		Form: WordFormValues{
			Word:            word.Word,
			Type:            word.Type,
			Meaning:         word.Meaning,
			Sentence:        word.Sentence,
			Origin:          word.Origin,
			ConfusedWithRaw: joinStringList(word.ConfusedWith),
			ConfusedWith:    word.ConfusedWith,
		},
	})
}

func (server *Server) postAdminEditWord(w http.ResponseWriter, req *http.Request) {
	id, err := strconv.Atoi(req.PathValue("id"))
	if err != nil {
		renderTemplate(w, "./frontend/404.html", nil)
		return
	}

	form, formErr := parseAndValidateWordForm(req)
	if formErr != "" {
		w.WriteHeader(http.StatusBadRequest)
		renderTemplate(w, "./frontend/admin_word_form.html", AdminWordFormPageData{
			Title:        "Edit Word",
			SubmitAction: fmt.Sprintf("/admin/word/%d/edit", id),
			SubmitLabel:  "Update Word",
			Error:        formErr,
			CSRFToken:    server.ensureCSRFToken(w, req),
			Form:         form,
		})
		return
	}

	word, err := server.dbQueries.UpdateWord(context.Background(), database.UpdateWordParams{
		ID:           int32(id),
		Word:         form.Word,
		Type:         form.Type,
		Meaning:      form.Meaning,
		Sentence:     form.Sentence,
		Origin:       form.Origin,
		ConfusedWith: form.ConfusedWith,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			renderTemplate(w, "./frontend/404.html", nil)
			return
		}
		log.Printf("update word db error: %v", err)
		http.Error(w, INTERNAL_ERROR, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, req, fmt.Sprintf("/word/%d", word.ID), http.StatusSeeOther)
}

func (server *Server) getApiHealth(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set(TYPE, TYPE_PLAIN)
	io.WriteString(w, "OK")
}
