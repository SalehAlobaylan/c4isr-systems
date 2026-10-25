// Package httpx contains HTTP response helpers and middleware shared by all
// module transports. It deliberately contains no domain behavior.
package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/presentation"
)

const maxBodyBytes = 1 << 20 // 1 MiB

// ErrorBody is the standard error envelope.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

// ListResponse is the standard shape for collection endpoints.
type ListResponse[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

// ErrorDetail describes a single error.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// JSON writes v as a JSON response with the given status.
func JSON(w http.ResponseWriter, status int, v any) {
	raw, err := responseBytes(w, v)
	if err != nil {
		http.Error(w, "response unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if _, err := w.Write(append(raw, '\n')); err != nil {
		slog.Default().Error("encode response", "error", err)
	}
}

// NoContent writes a 204 response.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Error maps err to an HTTP response. Internal errors never leak details.
func Error(w http.ResponseWriter, err error) {
	code := apperr.CodeOf(err)
	status := statusFor(code)

	message := ""
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		message = appErr.Message
	}
	if code == apperr.CodeInternal {
		message = "internal error"
		slog.Default().Error("request failed", "code", code)
	}

	JSON(w, status, ErrorBody{Error: ErrorDetail{Code: string(code), Message: message}})
}

func statusFor(code apperr.Code) int {
	switch code {
	case apperr.CodeValidation, apperr.CodeBadRequest:
		return http.StatusBadRequest
	case apperr.CodeNotFound:
		return http.StatusNotFound
	case apperr.CodeConflict:
		return http.StatusConflict
	case apperr.CodeUnauthorized:
		return http.StatusUnauthorized
	case apperr.CodeForbidden:
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}

// DecodeJSON decodes a JSON request body into dst, rejecting malformed input
// and bodies larger than 1 MiB.
func DecodeJSON(r *http.Request, dst any) error {
	body := http.MaxBytesReader(nil, r.Body, maxBodyBytes)
	dec := json.NewDecoder(body)
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return apperr.BadRequest("request body too large")
		}
		if errors.Is(err, io.EOF) {
			return apperr.BadRequest("request body is required")
		}
		return apperr.BadRequest("invalid JSON body")
	}
	return presentation.ResolveInput(r.Context(), dst, strings.HasSuffix(r.URL.Path, "/observations"))
}
