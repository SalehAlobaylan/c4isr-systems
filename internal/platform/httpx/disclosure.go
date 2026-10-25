package httpx

import (
	"bufio"
	"context"
	"net"
	"net/http"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/presentation"
)

type responseContext struct {
	http.ResponseWriter
	request *http.Request
}

func (w *responseContext) ResponseContext() (context.Context, string) {
	return w.request.Context(), w.request.URL.Path
}
func (w *responseContext) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *responseContext) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(w.ResponseWriter).Hijack()
}

// ResponseDisclosure binds JSON encoding to the authenticated resource. It
// wraps all domain handlers so list/detail/replay/ingress cannot diverge.
func ResponseDisclosure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { next.ServeHTTP(&responseContext{w, r}, r) })
}

func responseBytes(w http.ResponseWriter, v any) ([]byte, error) {
	ctx, path := context.Background(), ""
	if scoped, ok := w.(interface {
		ResponseContext() (context.Context, string)
	}); ok {
		ctx, path = scoped.ResponseContext()
	}
	return presentation.Encode(ctx, path, v)
}
