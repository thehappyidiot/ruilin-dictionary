package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
)

func TestParseStringList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{
			name: "empty input",
			raw:  "",
			want: []string{},
		},
		{
			name: "newline separated",
			raw:  "alpha\nbeta\ngamma",
			want: []string{"alpha", "beta", "gamma"},
		},
		{
			name: "comma separated",
			raw:  "alpha, beta, gamma",
			want: []string{"alpha", "beta", "gamma"},
		},
		{
			name: "mixed separators and whitespace",
			raw:  " alpha,\n\nbeta ,  gamma  ",
			want: []string{"alpha", "beta", "gamma"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseStringList(tt.raw)
			if len(got) != len(tt.want) {
				t.Fatalf("length mismatch: got=%v want=%v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("element mismatch at %d: got=%q want=%q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestJoinStringList(t *testing.T) {
	t.Parallel()

	if got := joinStringList([]string{}); got != "" {
		t.Fatalf("expected empty output, got %q", got)
	}

	if got := joinStringList([]string{"alpha", "beta"}); got != "alpha\nbeta" {
		t.Fatalf("unexpected joined output: %q", got)
	}
}

func TestParseAndValidateWordForm(t *testing.T) {
	t.Parallel()

	t.Run("valid form", func(t *testing.T) {
		t.Parallel()
		req := newFormRequest(url.Values{
			"word":         {"teacup"},
			"type":         {"noun"},
			"meaning":      {"a small cup"},
			"sentence":     {"I found your teacup."},
			"origin":       {"inside joke"},
			"confused_with": {"teapot,\n mug"},
		})

		gotForm, gotErr := parseAndValidateWordForm(req)
		if gotErr != "" {
			t.Fatalf("unexpected error: %s", gotErr)
		}

		if gotForm.Word != "teacup" || gotForm.Type != "noun" || gotForm.Origin != "inside joke" {
			t.Fatalf("unexpected parsed fields: %+v", gotForm)
		}
		wantConfusions := []string{"teapot", "mug"}
		if len(gotForm.ConfusedWith) != len(wantConfusions) {
			t.Fatalf("unexpected confusions: got=%v want=%v", gotForm.ConfusedWith, wantConfusions)
		}
		for i := range wantConfusions {
			if gotForm.ConfusedWith[i] != wantConfusions[i] {
				t.Fatalf("unexpected confusions: got=%v want=%v", gotForm.ConfusedWith, wantConfusions)
			}
		}
	})

	requiredFieldCases := []struct {
		name       string
		mutate     func(url.Values)
		wantErrMsg string
	}{
		{
			name: "missing word",
			mutate: func(v url.Values) {
				v.Set("word", "")
			},
			wantErrMsg: "Word is required",
		},
		{
			name: "missing type",
			mutate: func(v url.Values) {
				v.Set("type", "")
			},
			wantErrMsg: "Type is required",
		},
		{
			name: "missing meaning",
			mutate: func(v url.Values) {
				v.Set("meaning", "")
			},
			wantErrMsg: "Meaning is required",
		},
		{
			name: "missing sentence",
			mutate: func(v url.Values) {
				v.Set("sentence", "")
			},
			wantErrMsg: "Sentence is required",
		},
	}

	for _, tc := range requiredFieldCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			values := url.Values{
				"word":     {"teacup"},
				"type":     {"noun"},
				"meaning":  {"a small cup"},
				"sentence": {"I found your teacup."},
			}
			tc.mutate(values)

			_, gotErr := parseAndValidateWordForm(newFormRequest(values))
			if gotErr != tc.wantErrMsg {
				t.Fatalf("unexpected error: got=%q want=%q", gotErr, tc.wantErrMsg)
			}
		})
	}
}

func TestLoginRateLimiter_BackoffAndAllow(t *testing.T) {
	t.Parallel()

	limiter := newLoginRateLimiter(100*time.Millisecond, 2, 3)
	ip := "127.0.0.1"

	if !limiter.Allow(ip) {
		t.Fatal("expected first attempt to be allowed")
	}
	if got := limiter.Backoff(ip); got != 0 {
		t.Fatalf("expected no backoff before failures, got %s", got)
	}

	limiter.RecordFailure(ip)
	if got := limiter.Backoff(ip); got != 0 {
		t.Fatalf("expected no backoff after 1 failure, got %s", got)
	}

	limiter.RecordFailure(ip)
	if got := limiter.Backoff(ip); got != 250*time.Millisecond {
		t.Fatalf("expected 250ms backoff after 2 failures, got %s", got)
	}
	if limiter.Allow(ip) {
		t.Fatal("expected per-ip limit to block further attempts")
	}

	time.Sleep(120 * time.Millisecond)
	if !limiter.Allow(ip) {
		t.Fatal("expected attempts to be allowed after window passes")
	}
}

func TestLoginRateLimiter_BackoffCap(t *testing.T) {
	t.Parallel()

	limiter := newLoginRateLimiter(time.Minute, 100, 100)
	ip := "10.0.0.9"
	for range 20 {
		limiter.RecordFailure(ip)
	}

	if got := limiter.Backoff(ip); got != 2*time.Second {
		t.Fatalf("expected capped backoff of 2s, got %s", got)
	}
}

func TestVerifyAdminPassword(t *testing.T) {
	t.Parallel()

	hash, err := bcrypt.GenerateFromPassword([]byte("hunter2"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to build test hash: %v", err)
	}

	s := &Server{config: Config{adminPasswordHash: string(hash)}}
	if !s.verifyAdminPassword("hunter2") {
		t.Fatal("expected correct password to verify")
	}
	if s.verifyAdminPassword("wrong") {
		t.Fatal("expected wrong password to fail verification")
	}
}

func TestCSRFTokenLifecycle(t *testing.T) {
	t.Parallel()

	s := newTestServer()
	initialReq := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rr := httptest.NewRecorder()

	token := s.ensureCSRFToken(rr, initialReq)
	if token == "" {
		t.Fatal("expected csrf token")
	}

	cookie := firstCookie(rr)
	if cookie == nil {
		t.Fatal("expected session cookie to be set")
	}

	secondReq := httptest.NewRequest(http.MethodGet, "/admin", nil)
	secondReq.AddCookie(cookie)
	secondRR := httptest.NewRecorder()
	token2 := s.ensureCSRFToken(secondRR, secondReq)
	if token2 != token {
		t.Fatalf("expected same csrf token from session, got %q and %q", token, token2)
	}

	postReq := newFormRequest(url.Values{"csrf_token": {token}})
	postReq.AddCookie(cookie)
	if !s.validateCSRFToken(postReq) {
		t.Fatal("expected csrf validation to pass")
	}

	badReq := newFormRequest(url.Values{"csrf_token": {"wrong"}})
	badReq.AddCookie(cookie)
	if s.validateCSRFToken(badReq) {
		t.Fatal("expected csrf validation to fail for wrong token")
	}
}

func TestRequireAdmin(t *testing.T) {
	t.Parallel()

	s := newTestServer()
	nextCalled := false
	next := s.requireAdmin(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	}))

	unauthReq := httptest.NewRequest(http.MethodGet, "/admin/word/new", nil)
	unauthRR := httptest.NewRecorder()
	next.ServeHTTP(unauthRR, unauthReq)
	if status := unauthRR.Result().StatusCode; status != http.StatusSeeOther {
		t.Fatalf("expected redirect for non-admin, got %d", status)
	}
	if nextCalled {
		t.Fatal("expected next handler not to be called for non-admin")
	}

	loginReq := httptest.NewRequest(http.MethodGet, "/admin", nil)
	loginRR := httptest.NewRecorder()
	if !s.setAdmin(loginReq, loginRR, true) {
		t.Fatal("expected setAdmin to succeed")
	}
	cookie := firstCookie(loginRR)
	if cookie == nil {
		t.Fatal("expected admin session cookie")
	}

	adminReq := httptest.NewRequest(http.MethodGet, "/admin/word/new", nil)
	adminReq.AddCookie(cookie)
	adminRR := httptest.NewRecorder()
	next.ServeHTTP(adminRR, adminReq)
	if status := adminRR.Result().StatusCode; status != http.StatusNoContent {
		t.Fatalf("expected protected handler to run, got %d", status)
	}
	if !nextCalled {
		t.Fatal("expected next handler to be called for admin")
	}
}

func TestGetClientIP(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.5:32123"
	if got := getClientIP(req); got != "203.0.113.5" {
		t.Fatalf("unexpected parsed ip: %q", got)
	}

	req.RemoteAddr = "not-a-socket-address"
	if got := getClientIP(req); got != "not-a-socket-address" {
		t.Fatalf("expected fallback address, got %q", got)
	}
}

func newTestServer() *Server {
	store := sessions.NewCookieStore([]byte(strings.Repeat("a", 32)))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   3600,
		HttpOnly: true,
	}

	return &Server{
		config:       Config{adminPasswordHash: "$2a$12$F6QXzBf6J7A1QqVi8ceN1u1xy4fM6v7Ndr3U4FJGuOD8ibBqQj9Ku"},
		sessionStore: store,
	}
}

func newFormRequest(values url.Values) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/admin/test", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.ContentLength = int64(len(values.Encode()))
	return req
}

func firstCookie(rr *httptest.ResponseRecorder) *http.Cookie {
	res := rr.Result()
	defer res.Body.Close()

	cookies := res.Cookies()
	if len(cookies) == 0 {
		return nil
	}

	// Copy to avoid mutating slice-backed values from response internals.
	c := *cookies[0]
	return &c
}

func Example_parseStringList() {
	fmt.Println(parseStringList("alpha, beta\n gamma"))
	// Output: [alpha beta gamma]
}
