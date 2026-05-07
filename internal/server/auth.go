package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
)

const sessionName = "ruilin_dictionary_session"
const sessionAdminKey = "is_admin"
const sessionCSRFTokenKey = "csrf_token"

type loginRateLimiter struct {
	mu             sync.Mutex
	window         time.Duration
	perIPMax       int
	globalMax      int
	ipAttempts     map[string][]time.Time
	globalAttempts []time.Time
}

func newLoginRateLimiter(window time.Duration, perIPMax int, globalMax int) *loginRateLimiter {
	return &loginRateLimiter{
		window:         window,
		perIPMax:       perIPMax,
		globalMax:      globalMax,
		ipAttempts:     make(map[string][]time.Time),
		globalAttempts: make([]time.Time, 0),
	}
}

func (limiter *loginRateLimiter) Allow(ip string) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	now := time.Now()
	limiter.pruneLocked(now)

	return len(limiter.ipAttempts[ip]) < limiter.perIPMax && len(limiter.globalAttempts) < limiter.globalMax
}

func (limiter *loginRateLimiter) Backoff(ip string) time.Duration {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	attemptCount := len(limiter.ipAttempts[ip])
	if attemptCount < 2 {
		return 0
	}

	backoffStep := time.Duration(attemptCount-1) * 250 * time.Millisecond
	maxBackoff := 2 * time.Second
	if backoffStep > maxBackoff {
		return maxBackoff
	}
	return backoffStep
}

func (limiter *loginRateLimiter) RecordFailure(ip string) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	now := time.Now()
	limiter.pruneLocked(now)
	limiter.ipAttempts[ip] = append(limiter.ipAttempts[ip], now)
	limiter.globalAttempts = append(limiter.globalAttempts, now)
}

func (limiter *loginRateLimiter) RecordSuccess(ip string) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	delete(limiter.ipAttempts, ip)
}

func (limiter *loginRateLimiter) pruneLocked(now time.Time) {
	threshold := now.Add(-limiter.window)

	for ip, attempts := range limiter.ipAttempts {
		filtered := attempts[:0]
		for _, attempt := range attempts {
			if attempt.After(threshold) {
				filtered = append(filtered, attempt)
			}
		}
		if len(filtered) == 0 {
			delete(limiter.ipAttempts, ip)
			continue
		}
		limiter.ipAttempts[ip] = filtered
	}

	filteredGlobal := limiter.globalAttempts[:0]
	for _, attempt := range limiter.globalAttempts {
		if attempt.After(threshold) {
			filteredGlobal = append(filteredGlobal, attempt)
		}
	}
	limiter.globalAttempts = filteredGlobal
}

func (server *Server) verifyAdminPassword(password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(server.config.adminPasswordHash), []byte(password)) == nil
}

func (server *Server) getSession(req *http.Request) (*sessions.Session, error) {
	session, err := server.sessionStore.Get(req, sessionName)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (server *Server) isAdmin(req *http.Request) bool {
	session, err := server.getSession(req)
	if err != nil {
		return false
	}

	isAdmin, ok := session.Values[sessionAdminKey].(bool)
	return ok && isAdmin
}

func (server *Server) setAdmin(req *http.Request, w http.ResponseWriter, isAdmin bool) bool {
	session, err := server.getSession(req)
	if err != nil {
		log.Printf("session read error: %v", err)
		http.Error(w, INTERNAL_ERROR, http.StatusInternalServerError)
		return false
	}

	if isAdmin {
		session.Values[sessionAdminKey] = true
	} else {
		delete(session.Values, sessionAdminKey)
	}

	if err := session.Save(req, w); err != nil {
		log.Printf("session save error: %v", err)
		http.Error(w, INTERNAL_ERROR, http.StatusInternalServerError)
		return false
	}

	return true
}

func (server *Server) ensureCSRFToken(w http.ResponseWriter, req *http.Request) string {
	session, err := server.getSession(req)
	if err != nil {
		log.Printf("session read error: %v", err)
		http.Error(w, INTERNAL_ERROR, http.StatusInternalServerError)
		return ""
	}

	if existing, ok := session.Values[sessionCSRFTokenKey].(string); ok && existing != "" {
		return existing
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		log.Printf("csrf token generation error: %v", err)
		http.Error(w, INTERNAL_ERROR, http.StatusInternalServerError)
		return ""
	}

	token := base64.RawURLEncoding.EncodeToString(buf)
	session.Values[sessionCSRFTokenKey] = token
	if err := session.Save(req, w); err != nil {
		log.Printf("session save error: %v", err)
		http.Error(w, INTERNAL_ERROR, http.StatusInternalServerError)
		return ""
	}

	return token
}

func (server *Server) validateCSRFToken(req *http.Request) bool {
	session, err := server.getSession(req)
	if err != nil {
		return false
	}

	expected, ok := session.Values[sessionCSRFTokenKey].(string)
	if !ok || expected == "" {
		return false
	}

	submitted := req.FormValue("csrf_token")
	if submitted == "" {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(expected), []byte(submitted)) == 1
}

func (server *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !server.isAdmin(req) {
			http.Redirect(w, req, "/admin", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, req)
	})
}

func (server *Server) requireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !server.validateCSRFToken(req) {
			http.Error(w, "Invalid CSRF token", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, req)
	})
}

func parseAndValidateWordForm(req *http.Request) (WordFormValues, string) {
	form := WordFormValues{
		Word:     strings.TrimSpace(req.FormValue("word")),
		Type:     strings.TrimSpace(req.FormValue("type")),
		Meaning:  strings.TrimSpace(req.FormValue("meaning")),
		Sentence: strings.TrimSpace(req.FormValue("sentence")),
		Origin:   strings.TrimSpace(req.FormValue("origin")),
	}

	switch {
	case form.Word == "":
		return form, "Word is required"
	case form.Type == "":
		return form, "Type is required"
	case form.Meaning == "":
		return form, "Meaning is required"
	case form.Sentence == "":
		return form, "Sentence is required"
	default:
		return form, ""
	}
}

func getClientIP(req *http.Request) string {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return req.RemoteAddr
	}
	return host
}
