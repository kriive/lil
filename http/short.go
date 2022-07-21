package http

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/kriive/lil"
	"github.com/kriive/lil/generate"
)

func (s *Server) registerShortRoutes(r chi.Router) {
	r.Post("/shorten", s.handleShortenURL())
	r.Handle("/s/{key}", s.handleShortenedURL())
}

func (s *Server) handleShortenURL() http.HandlerFunc {
	type ShortenRequest struct {
		URL string `json:"url"`
	}
	type ShortenResponse struct {
		Key          string `json:"key"`
		ShortenedURL string `json:"shortened_url"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		req := &ShortenRequest{}
		if err := json.NewDecoder(r.Body).Decode(req); err != nil {
			Error(w, r, lil.Errorf(lil.EINVALID, "We couldn't parse the request body."))
			return
		}

		uri, err := url.ParseRequestURI(req.URL)
		if err != nil {
			Error(w, r, lil.Errorf(lil.EINVALID, "The URL you provided is not valid."))
			return
		}

		if uri.Scheme != "http" && uri.Scheme != "https" {
			Error(w, r, lil.Errorf(lil.EINVALID, "The URL scheme you provided is not supported. Only HTTP and HTTPS are supported."))
			return
		}

		key, err := generate.SecureStringFromAlphabet(s.KeyLength, s.Alphabet)
		if err != nil {
			Error(w, r, err)
			return
		}

		short := &lil.Short{URL: *uri, Key: key}
		if err := s.ShortService.CreateShort(r.Context(), short); err != nil {
			Error(w, r, err)
			return
		}

		if err := json.NewEncoder(w).Encode(&ShortenResponse{Key: key, ShortenedURL: s.URL() + "/" + key}); err != nil {
			Error(w, r, err)
			return
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
