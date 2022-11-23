package http

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/kriive/lil"
	"github.com/kriive/lil/generate"
)

func (s *Server) registerShortPublicRoutes(r chi.Router) {
	r.Handle("/s/{key}", s.handleShortenedURL())
}

func (s *Server) registerShortPrivateRoutes(r chi.Router) {
	r.Post("/short/new", s.handleShortURLCreate())
	r.Get("/short/new", s.handleShortURLNew())
	r.Delete("/s/{key}", s.handleShortURLDelete())
	r.Get("/short", s.handleShortsIndex())
}

// handleShortURLNew handles the "GET /short/new" route.
// It renders an HTML form for editing a new dial.
func (s *Server) handleShortURLNew() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := s.Views.ShortView.Render(w, r, nil); err != nil {
			Error(w, r, err)
			return
		}
	}
}

// handleShortURLDelete handles the "DELETE /s/{key}" route.
// It deletes a short, if the user is the owner.
func (s *Server) handleShortURLDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := chi.URLParam(r, "key")
		if key == "" {
			Error(w, r, lil.Errorf(lil.EINTERNAL, "empty URLParameter: shouldn't happen, did you forget to configure the route?"))
			return
		}

		if err := s.ShortService.DeleteShort(r.Context(), key); err != nil {
			Error(w, r, err)
			return
		}

		SetFlash(w, "Successfully deleted short "+key+".")
		http.Redirect(w, r, "/short", http.StatusFound)
	}
}

// handleShortURLCreate handles the "POST /short/new" route.
// It reads & writes data using HTML or JSON, depending on
// HTTP Accept Header.
func (s *Server) handleShortURLCreate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		short := &lil.Short{}

		// Format returned data based on HTTP accept header.
		switch r.Header.Get("Accept") {
		case "application/json":
			if err := json.NewDecoder(r.Body).Decode(short); err != nil {
				Error(w, r, lil.Errorf(lil.EINVALID, "We couldn't parse the request body."))
				return
			}
		default:
			url, err := url.ParseRequestURI(r.FormValue("url"))
			if err != nil {
				Error(w, r, lil.Errorf(lil.EINVALID, "Invalid URL passed."))
				return
			}

			short.URL = *url
		}

		var err error

		// Overwrite the possibly user-provided Key to avoid
		// attacks and vanity URLs. A future version may actually
		// permit those.
		short.Key, err = generate.SecureStringFromAlphabet(s.KeyLength, s.Alphabet)
		if err != nil {
			Error(w, r, err)
			return
		}

		if err := s.ShortService.CreateShort(r.Context(), short); err != nil {
			Error(w, r, err)
			return
		}

		switch r.Header.Get("Accept") {
		case "application/json":
			if err := json.NewEncoder(w).Encode(short); err != nil {
				Error(w, r, err)
				return
			}
		default:
			// This is the HTML one
			if err := s.Views.NewShort.Render(w, r,
				struct {
					ShortURL    string
					OriginalURL string
				}{
					ShortURL:    s.URL() + "/s/" + short.Key,
					OriginalURL: short.URL.String(),
				}); err != nil {
				Error(w, r, err)
				return
			}
		}
	}
}

func (s *Server) handleShortsIndex() http.HandlerFunc {
	// findShortsResponse represents the output JSON struct for "GET /short".
	type findShortsResponse struct {
		Shorts []*lil.Short `json:"shorts"`
		N      int          `json:"n"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse optional filter object.
		var filter lil.ShortFilter
		switch r.Header.Get("Content-type") {
		case "application/json":
			if err := json.NewDecoder(r.Body).Decode(&filter); err != nil {
				Error(w, r, lil.Errorf(lil.EINVALID, "Invalid JSON body"))
				return
			}
		default:
			filter.Offset, _ = strconv.Atoi(r.URL.Query().Get("offset"))
			filter.Limit = 20
		}

		// Fetch shorts from database.
		shorts, n, err := s.ShortService.FindShorts(r.Context(), filter)
		if err != nil {
			Error(w, r, err)
			return
		}

		// Render output based on HTTP accept header.
		switch r.Header.Get("Accept") {
		case "application/json":
			w.Header().Set("Content-type", "application/json")
			if err := json.NewEncoder(w).Encode(findShortsResponse{
				Shorts: shorts,
				N:      n,
			}); err != nil {
				LogError(r, err)
				return
			}

		default:
			if err := s.Views.ShortsIndexView.Render(w, r, struct {
				Shorts []*lil.Short
				N      int
				Filter lil.ShortFilter
			}{
				Shorts: shorts,
				N:      n,
				Filter: filter,
			}); err != nil {
				Error(w, r, err)
				return
			}
		}
	}
}

func (s *Server) handleShortenedURL() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := chi.URLParam(r, "key")
		if key == "" {
			Error(w, r, lil.Errorf(lil.EINTERNAL, "empty URLParameter: shouldn't happen, did you forget to configure the route?"))
			return
		}

		short, err := s.ShortService.FindShortByKey(r.Context(), key)
		if err != nil {
			Error(w, r, err)
			return
		}

		http.Redirect(w, r, short.URL.String(), http.StatusMovedPermanently)
	}
}
