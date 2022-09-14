package http

import (
	"context"
	"encoding/hex"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/securecookie"
	"github.com/kriive/lil"
	"github.com/kriive/lil/http/assets"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

const ShutdownTimeout = time.Second * 5

type Server struct {
	ln     net.Listener
	server *http.Server
	router chi.Router
	sc     *securecookie.SecureCookie

	// Bind address & domain for the server's listener.
	// If domain is specified, server is run on TLS using acme/autocert.
	Addr   string
	Domain string

	// Keys used for secure cookie encryption.
	HashKey  string
	BlockKey string

	// GitHub OAuth settings.
	GitHubClientID     string
	GitHubClientSecret string

	// Google OAuth settings.
	GoogleClientID     string
	GoogleClientSecret string

	// Link length
	KeyLength int
	Alphabet  string

	// Services used by the various HTTP routes.
	AuthService  lil.AuthService
	ShortService lil.ShortService
	UserService  lil.UserService
}

func NewServer() *Server {
	s := &Server{
		server: &http.Server{},
		router: chi.NewRouter(),
	}

	s.server.Handler = s.router

	// Setup endpoint to display deployed version.
	s.router.Get("/debug/version", s.handleVersion)
	s.router.Get("/debug/commit", s.handleCommit)

	router := chi.NewRouter()
	router.Use(s.authenticate)
	router.Use(loadFlash)

	router.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assets.FS))))

	// Unauthenticated routes
	router.Group(func(r chi.Router) {
		s.registerAuthRoutes(r)
		s.registerShortPublicRoutes(r)
	})

	// Authenticated routes
	router.Group(func(r chi.Router) {
		r.Use(s.requireAuth)
		s.registerShortPrivateRoutes(r)
	})

	s.router.Mount("/", router)
	s.router.Get("/", s.handleIndex)

	return s
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if lil.UserIDFromContext(r.Context()) == 0 {
		files := []string{
			"templates/base.tmpl.html",
			"templates/index/index.tmpl.html",
		}

		tmpl, err := template.ParseFS(templates, files...)
		if err != nil {
			Error(w, r, err)
			return
		}

		if err := tmpl.ExecuteTemplate(w, "base", nil); err != nil {
			Error(w, r, err)
			return
		}
	}
}

// handleVersion displays the deployed version.
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(lil.Version))
}

// handleVersion displays the deployed commit.
func (s *Server) handleCommit(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(lil.Commit))
}

// UseTLS returns true if the cert & key file are specified.
func (s *Server) UseTLS() bool {
	return s.Domain != ""
}

// Scheme returns the URL scheme for the server.
func (s *Server) Scheme() string {
	if s.UseTLS() {
		return "https"
	}
	return "http"
}

// Port returns the TCP port for the running server.
// This is useful in tests where we allocate a random port by using ":0".
func (s *Server) Port() int {
	if s.ln == nil {
		return 0
	}
	return s.ln.Addr().(*net.TCPAddr).Port
}

// URL returns the local base URL of the running server.
func (s *Server) URL() string {
	scheme, port := s.Scheme(), s.Port()

	// Use localhost unless a domain is specified.
	domain := "localhost"
	if s.Domain != "" {
		domain = s.Domain
	}

	// Return without port if using standard ports.
	if (scheme == "http" && port == 80) || (scheme == "https" && port == 443) {
		return fmt.Sprintf("%s://%s", s.Scheme(), domain)
	}

	return fmt.Sprintf("%s://%s:%d", s.Scheme(), domain, s.Port())
}

// Close gracefully shuts down the server.
func (s *Server) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
	defer cancel()
	return s.server.Shutdown(ctx)
}

// OAuth2Config returns the GitHub OAuth2 configuration.
func (s *Server) OAuth2Config(provider string) *oauth2.Config {
	switch provider {
	case lil.AuthSourceGitHub:
		return &oauth2.Config{
			ClientID:     s.GitHubClientID,
			ClientSecret: s.GitHubClientSecret,
			Scopes:       []string{},
			Endpoint:     github.Endpoint,
		}
	case lil.AuthSourceGoogle:
		return &oauth2.Config{
			ClientID:     s.GoogleClientID,
			ClientSecret: s.GoogleClientSecret,
			Scopes: []string{
				"openid",
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
			},
			Endpoint: google.Endpoint,
		}
	}

	return nil
}

func (s *Server) Open() (err error) {
	// Initialize our secure cookie with our encryption keys.
	if err := s.openSecureCookie(); err != nil {
		return err
	}

	if s.GitHubClientID == "" {
		return fmt.Errorf("github client id required")
	} else if s.GitHubClientSecret == "" {
		return fmt.Errorf("github client secret required")
	}

	if s.GoogleClientID == "" {
		return fmt.Errorf("google client id required")
	} else if s.GoogleClientSecret == "" {
		return fmt.Errorf("google client secret required")
	}

	// Open a listener on our bind address.
	if s.Domain != "" {
		s.ln = autocert.NewListener(s.Domain)
	} else {
		if s.ln, err = net.Listen("tcp", s.Addr); err != nil {
			return err
		}
	}

	// Begin serving requests on the listener. We use Serve() instead of
	// ListenAndServe() because it allows us to check for listen errors (such
	// as trying to use an already open port) synchronously.
	go s.server.Serve(s.ln)

	return nil
}

// openSecureCookie validates & decodes the block & hash key and initializes
// our secure cookie implementation.
func (s *Server) openSecureCookie() error {
	// Ensure hash & block key are set.
	if s.HashKey == "" {
		return fmt.Errorf("hash key required")
	} else if s.BlockKey == "" {
		return fmt.Errorf("block key required")
	}

	// Decode from hex to byte slices.
	hashKey, err := hex.DecodeString(s.HashKey)
	if err != nil {
		return fmt.Errorf("invalid hash key")
	}
	blockKey, err := hex.DecodeString(s.BlockKey)
	if err != nil {
		return fmt.Errorf("invalid block key")
	}

	// Initialize cookie management & encode our cookie data as JSON.
	s.sc = securecookie.New(hashKey, blockKey)
	s.sc.SetSerializer(securecookie.JSONEncoder{})

	return nil
}

// session returns session data from the secure cookie.
func (s *Server) session(r *http.Request) (Session, error) {
	// Read session data from cookie.
	// If it returns an error then simply return an empty session.
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return Session{}, nil
	}

	// Decode session data into a Session object & return.
	var session Session
	if err := s.UnmarshalSession(cookie.Value, &session); err != nil {
		return Session{}, err
	}
	return session, nil
}

// setSession creates a secure cookie with session data.
func (s *Server) setSession(w http.ResponseWriter, session Session) error {
	// Encode session data to JSON.
	buf, err := s.MarshalSession(session)
	if err != nil {
		return err
	}

	// Write cookie to HTTP response.
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    buf,
		Path:     "/",
		Expires:  time.Now().Add(30 * 24 * time.Hour),
		Secure:   s.UseTLS(),
		HttpOnly: true,
	})
	return nil
}

// MarshalSession encodes session data to string.
// This is exported to allow the unit tests to generate fake sessions.
func (s *Server) MarshalSession(session Session) (string, error) {
	return s.sc.Encode(SessionCookieName, session)
}

// UnmarshalSession decodes session data into a Session object.
// This is exported to allow the unit tests to generate fake sessions.
func (s *Server) UnmarshalSession(data string, session *Session) error {
	return s.sc.Decode(SessionCookieName, data, &session)
}

// ListenAndServeTLSRedirect runs an HTTP server on port 80 to redirect users
// to the TLS-enabled port 443 server.
func ListenAndServeTLSRedirect(domain string) error {
	return http.ListenAndServe(":80", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://"+domain, http.StatusFound)
	}))
}

// authenticate is middleware for loading session data from a cookie or API key header.
func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Login via API key, if available.
		if v := r.Header.Get("Authorization"); strings.HasPrefix(v, "Bearer ") {
			apiKey := strings.TrimPrefix(v, "Bearer ")

			// Lookup user by API key. Display error if not found.
			// Otherwise set
			users, _, err := s.UserService.FindUsers(r.Context(), lil.UserFilter{APIKey: &apiKey})
			if err != nil {
				Error(w, r, err)
				return
			} else if len(users) == 0 {
				Error(w, r, lil.Errorf(lil.EUNAUTHORIZED, "Invalid API key."))
				return
			}

			// Update request context to include authenticated user.
			r = r.WithContext(lil.NewContextWithUser(r.Context(), users[0]))

			// Delegate to next HTTP handler.
			next.ServeHTTP(w, r)
			return
		}

		// Read session from secure cookie.
		session, _ := s.session(r)

		// Read user, if available. Ignore if fetching assets.
		if session.UserID != 0 {
			if user, err := s.UserService.FindUserByID(r.Context(), session.UserID); err != nil {
				log.Printf("cannot find session user: id=%d err=%s", session.UserID, err)
			} else {
				r = r.WithContext(lil.NewContextWithUser(r.Context(), user))
			}
		}

		next.ServeHTTP(w, r)
	})
}

// requireNoAuth is middleware for requiring no authentication.
// This is used if a user goes to log in but is already logged in.
func (s *Server) requireNoAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If user is logged in, redirect to the home page.
		if userID := lil.UserIDFromContext(r.Context()); userID != 0 {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		// Delegate to next HTTP handler.
		next.ServeHTTP(w, r)
	})
}

// requireAuth is middleware for requiring authentication. This is used by
// nearly every page except for the login & oauth pages.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If user is logged in, delegate to next HTTP handler.
		if userID := lil.UserIDFromContext(r.Context()); userID != 0 {
			next.ServeHTTP(w, r)
			return
		}

		// Otherwise save the current URL (without scheme/host).
		redirectURL := r.URL
		redirectURL.Scheme, redirectURL.Host = "", ""

		// Save the URL to the session and redirect to the log in page.
		// On successful login, the user will be redirected to their original location.
		session, _ := s.session(r)
		session.RedirectURL = redirectURL.String()
		if err := s.setSession(w, session); err != nil {
			log.Printf("http: cannot set session: %s", err)
		}
		http.Redirect(w, r, "/login", http.StatusFound)
	})
}

// loadFlash is middleware for reading flash data from the cookie.
// Data is only loaded once and then immediately cleared... hence the name "flash".
func loadFlash(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read & clear flash from cookies.
		if cookie, _ := r.Cookie("flash"); cookie != nil {
			SetFlash(w, "")
			r = r.WithContext(lil.NewContextWithFlash(r.Context(), cookie.Value))
		}

		// Delegate to next HTTP handler.
		next.ServeHTTP(w, r)
	})
}
