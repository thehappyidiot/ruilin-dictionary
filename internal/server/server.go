package server

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/sessions"
	_ "github.com/joho/godotenv/autoload"
	_ "github.com/lib/pq"
	"github.com/thehappyidiot/ruilin-dictionary/internal/database"
)

type Config struct {
	port              int
	isDevelopment     bool
	adminPasswordHash string
	sessionMaxAge     int
}

type Server struct {
	config           Config
	dbQueries        *database.Queries
	sessionStore     *sessions.CookieStore
	adminRateLimiter *loginRateLimiter
}

func NewServer() *http.Server {
	// Get config:
	port, err := strconv.Atoi(os.Getenv("PORT"))
	if err != nil {
		panic("Cannot parse environment variable `port` as int")
	}

	isDevelopment := false
	if os.Getenv("IS_DEVELOPMENT") != "" {
		isDevelopment, err = strconv.ParseBool(os.Getenv("IS_DEVELOPMENT"))
		if err != nil {
			panic("Cannot parse environment variable `is_development` as boolean")
		}
	}

	if isDevelopment {
		fmt.Print("Server is running in Development mode. Do NOT use in Production. Speak friend and enter: ")
		var confirmation string
		fmt.Scanln(&confirmation)
		if "mellon" != strings.ToLower(confirmation) {
			panic("You shall not pass 🧙")
		}
	}

	adminPasswordHash := os.Getenv("ADMIN_PASSWORD_HASH")
	if adminPasswordHash == "" {
		panic("Missing required environment variable `ADMIN_PASSWORD_HASH`")
	}

	sessionSecret := os.Getenv("SESSION_SECRET")
	if len(sessionSecret) < 32 {
		panic("Environment variable `SESSION_SECRET` must be at least 32 characters long")
	}

	sessionMaxAgeSeconds := 8 * 60 * 60
	if rawSessionMaxAgeSeconds := os.Getenv("SESSION_MAX_AGE_SECONDS"); rawSessionMaxAgeSeconds != "" {
		sessionMaxAgeSeconds, err = strconv.Atoi(rawSessionMaxAgeSeconds)
		if err != nil || sessionMaxAgeSeconds <= 0 {
			panic("Cannot parse environment variable `SESSION_MAX_AGE_SECONDS` as positive int")
		}
	}

	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		panic(fmt.Sprintf("cannot connect to database: %s", err))
	}

	sessionStore := sessions.NewCookieStore([]byte(sessionSecret))
	sessionStore.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   sessionMaxAgeSeconds,
		HttpOnly: true,
		Secure:   !isDevelopment,
		SameSite: http.SameSiteLaxMode,
	}

	server := Server{
		config: Config{
			port:              port,
			isDevelopment:     isDevelopment,
			adminPasswordHash: adminPasswordHash,
			sessionMaxAge:     sessionMaxAgeSeconds,
		},
		dbQueries:        database.New(db),
		sessionStore:     sessionStore,
		adminRateLimiter: newLoginRateLimiter(15*time.Minute, 6, 30),
	}

	httpServer := &http.Server{
		Handler: server.RegisterRoutes(),
		Addr:    fmt.Sprintf(":%d", port),
	}

	return httpServer
}
