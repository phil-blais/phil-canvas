// Package web holds small shared HTTP helpers for JSON responses and request
// decoding, so handler packages share one response convention.
package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// WriteJSON writes body as JSON with the given status code.
func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// WriteError writes a JSON error envelope: {"error": msg}.
func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, map[string]string{"error": msg})
}

// DecodeJSON decodes a JSON request body into dst, rejecting unknown fields and
// bodies larger than 1 MiB. An empty body is treated as an empty object (dst is
// left zero-valued), so endpoints with only optional fields accept no body. On
// malformed input it writes a 400 and returns false.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil && !errors.Is(err, io.EOF) {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}
