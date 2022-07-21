package http

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/kriive/lil"
)

// Error prints & optionally logs an error message.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	// Extract error code & message.
	code, message := lil.ErrorCode(err), lil.ErrorMessage(err)

	// Log & report internal errors.
	if code == lil.EINTERNAL {
		LogError(r, err)
	}

	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(ErrorStatusCode(code))
	json.NewEncoder(w).Encode(&ErrorResponse{Error: message})
}

// ErrorResponse represents a JSON structure for error output.
type ErrorResponse struct {
	Error string `json:"error"`
}

// LogError logs an error with the HTTP route information.
func LogError(r *http.Request, err error) {
	log.Printf("[http] error: %s %s: %s", r.Method, r.URL.Path, err)
}

// lookup of application error codes to HTTP status codes.
var codes = map[string]int{
	lil.ECONFLICT:       http.StatusConflict,
	lil.EINVALID:        http.StatusBadRequest,
	lil.ENOTFOUND:       http.StatusNotFound,
	lil.ENOTIMPLEMENTED: http.StatusNotImplemented,
	lil.EUNAUTHORIZED:   http.StatusUnauthorized,
	lil.EINTERNAL:       http.StatusInternalServerError,
}

// ErrorStatusCode returns the associated HTTP status code for a lil error code.
func ErrorStatusCode(code string) int {
	if v, ok := codes[code]; ok {
		return v
	}
	return http.StatusInternalServerError
}

// FromErrorStatusCode returns the associated lil code for an HTTP status code.
func FromErrorStatusCode(code int) string {
	for k, v := range codes {
		if v == code {
			return k
		}
	}
	return lil.EINTERNAL
}