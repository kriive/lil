package http

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/go-github/v45/github"
	"github.com/kriive/lil"
	"golang.org/x/oauth2"
	google "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

// registerAuthRoutes is a helper function to register routes to a router.
func (s *Server) registerAuthRoutes(r chi.Router) {
	r.Get("/login", s.handleLogin)
	r.Delete("/logout", s.handleLogout)
	r.Get("/oauth/github", s.handleOAuthGitHub)
	r.Get("/oauth/github/callback", s.handleOAuthGitHubCallback)
	r.Get("/oauth/google", s.handleOAuthGoogle)
	r.Get("/oauth/google/callback", s.handleOAuthGoogleCallback)
}

// handleLogin handles the "GET /login" route. It simply renders an HTML login form.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	files := []string{
		"templates/base.tmpl.html",
		"templates/login/login.tmpl.html",
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

// handleLogout handles the "DELETE /logout" route. It clears the session
// cookie and redirects the user to the home page.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	// Clear session cookie on HTTP response.
	if err := s.setSession(w, Session{}); err != nil {
		Error(w, r, err)
		return
	}

	// Send user to the home page.
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) handleOAuthGoogle(w http.ResponseWriter, r *http.Request) {
	session, err := s.session(r)
	if err != nil {
		Error(w, r, err)
		return
	}

	state := make([]byte, 64)
	if _, err := io.ReadFull(rand.Reader, state); err != nil {
		Error(w, r, err)
		return
	}
	session.State = hex.EncodeToString(state)

	// Store the state to the session in the response cookie.
	if err := s.setSession(w, session); err != nil {
		Error(w, r, err)
		return
	}

	// Redirect to OAuth2 provider.
	http.Redirect(w, r, s.OAuth2Config(lil.AuthSourceGoogle).AuthCodeURL(session.State,
		oauth2.SetAuthURLParam("redirect_uri", s.URL()+"/oauth/google/callback")),
		http.StatusFound)
}

// handleOAuthGitHub handles the "GET /oauth/github" route. It generates a
// random state variable and redirects the user to the GitHub OAuth endpoint.
//
// After authentication, user will be redirected back to the callback page
// where we can store the returned OAuth tokens.
func (s *Server) handleOAuthGitHub(w http.ResponseWriter, r *http.Request) {
	// Read session from request's cookies.
	session, err := s.session(r)
	if err != nil {
		Error(w, r, err)
		return
	}

	// Generate new OAuth state for the session to prevent CSRF attacks.
	state := make([]byte, 64)
	if _, err := io.ReadFull(rand.Reader, state); err != nil {
		Error(w, r, err)
		return
	}
	session.State = hex.EncodeToString(state)

	// Store the state to the session in the response cookie.
	if err := s.setSession(w, session); err != nil {
		Error(w, r, err)
		return
	}

	// Redirect to OAuth2 provider.
	http.Redirect(w, r, s.OAuth2Config(lil.AuthSourceGitHub).AuthCodeURL(session.State), http.StatusFound)
}

func (s *Server) handleOAuthGoogleCallback(w http.ResponseWriter, r *http.Request) {
	state, code := r.FormValue("state"), r.FormValue("code")

	session, err := s.session(r)
	if err != nil {
		Error(w, r, fmt.Errorf("cannot read session: %s", err))
		return
	}

	if state != session.State {
		Error(w, r, fmt.Errorf("oauth state mismatch"))
		return
	}

	tok, err := s.OAuth2Config(lil.AuthSourceGoogle).Exchange(r.Context(),
		code,
		oauth2.SetAuthURLParam("redirect_uri", s.URL()+"/oauth/google/callback"))
	if err != nil {
		Error(w, r, fmt.Errorf("oauth exchange error: %s", err))
		return
	}

	g, err := google.NewService(r.Context(), option.WithTokenSource(oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: tok.AccessToken},
	)))
	if err != nil {
		Error(w, r, fmt.Errorf("google client creation error: %s", err))
		return
	}

	c, err := google.NewUserinfoV2Service(g).Me.Get().Do()
	if err != nil {
		Error(w, r, fmt.Errorf("cannot fetch google user: %s", err))
		return
	} else if c.Id == "" {
		Error(w, r, fmt.Errorf("user ID not returned by Google, cannot authenticate user."))
		return
	}

	// Create an authentication object with an associated user.
	auth := &lil.Auth{
		Source:       lil.AuthSourceGitHub,
		SourceID:     c.Id,
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		User: &lil.User{
			Name:  c.Name,
			Email: c.Email,
		},
	}
	if !tok.Expiry.IsZero() {
		auth.Expiry = &tok.Expiry
	}

	// Create the "Auth" object in the database. The AuthService will lookup
	// the user by email if they already exist. Otherwise, a new user will be
	// created and the user's ID will be set to auth.UserID.
	if err := s.AuthService.CreateAuth(r.Context(), auth); err != nil {
		Error(w, r, fmt.Errorf("cannot create auth: %s", err))
		return
	}

	// Restore redirect URL stored on login.
	redirectURL := session.RedirectURL

	// Update browser session to store the user's ID and clear OAuth state.
	session.UserID = auth.UserID
	session.RedirectURL = ""
	session.State = ""
	if err := s.setSession(w, session); err != nil {
		Error(w, r, fmt.Errorf("cannot set session cookie: %s", err))
		return
	}

	// Redirect to stored URL or, if not available, to the home page.
	if redirectURL == "" {
		redirectURL = "/"
	}
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// handleOAuthGitHubCallback handles the "GET /oauth/github/callback" route.
// It validates the returned OAuth state that we generated previously, looks up
// the current user's information, and creates an "Auth" object in the database.
func (s *Server) handleOAuthGitHubCallback(w http.ResponseWriter, r *http.Request) {
	// Read form variables passed in from GitHub.
	state, code := r.FormValue("state"), r.FormValue("code")

	// Read session from request.
	session, err := s.session(r)
	if err != nil {
		Error(w, r, fmt.Errorf("cannot read session: %s", err))
		return
	}

	// Validate that state matches session state.
	if state != session.State {
		Error(w, r, fmt.Errorf("oauth state mismatch"))
		return
	}

	// Exchange code for OAuth tokens.
	tok, err := s.OAuth2Config(lil.AuthSourceGitHub).Exchange(r.Context(), code)
	if err != nil {
		Error(w, r, fmt.Errorf("oauth exchange error: %s", err))
		return
	}

	// Create a new GitHub API client.
	client := github.NewClient(oauth2.NewClient(r.Context(), oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: tok.AccessToken},
	)))

	// Fetch user information for the currently authenticated user.
	// Require that we at least receive a user ID from GitHub.
	u, _, err := client.Users.Get(r.Context(), "")
	if err != nil {
		Error(w, r, fmt.Errorf("cannot fetch github user: %s", err))
		return
	} else if u.ID == nil {
		Error(w, r, fmt.Errorf("user ID not returned by GitHub, cannot authenticate user"))
		return
	}

	// Email is not necessarily available for all accounts. If it is, store it
	// so we can link together multiple OAuth providers in the future
	// (e.g. GitHub, Google, etc).
	var name string
	if u.Name != nil {
		name = *u.Name
	} else if u.Login != nil {
		name = *u.Login
	}
	var email string
	if u.Email != nil {
		email = *u.Email
	}

	// Create an authentication object with an associated user.
	auth := &lil.Auth{
		Source:       lil.AuthSourceGitHub,
		SourceID:     strconv.FormatInt(*u.ID, 10),
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		User: &lil.User{
			Name:  name,
			Email: email,
		},
	}
	if !tok.Expiry.IsZero() {
		auth.Expiry = &tok.Expiry
	}

	// Create the "Auth" object in the database. The AuthService will lookup
	// the user by email if they already exist. Otherwise, a new user will be
	// created and the user's ID will be set to auth.UserID.
	if err := s.AuthService.CreateAuth(r.Context(), auth); err != nil {
		Error(w, r, fmt.Errorf("cannot create auth: %s", err))
		return
	}

	// Restore redirect URL stored on login.
	redirectURL := session.RedirectURL

	// Update browser session to store the user's ID and clear OAuth state.
	session.UserID = auth.UserID
	session.RedirectURL = ""
	session.State = ""
	if err := s.setSession(w, session); err != nil {
		Error(w, r, fmt.Errorf("cannot set session cookie: %s", err))
		return
	}

	// Redirect to stored URL or, if not available, to the home page.
	if redirectURL == "" {
		redirectURL = "/"
	}
	http.Redirect(w, r, redirectURL, http.StatusFound)
}
