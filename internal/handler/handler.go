package handler

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net"
	"net/http"
)

// Updater is the interface the handler depends on.
type Updater interface {
	Update(ctx context.Context, prefix string) error
}

// Update handles GET /update from the Fritz!Box DynDNS mechanism.
// Expected query params:
//   - token        — must match the configured secret
//   - ip6lanprefix — IPv6 LAN prefix in CIDR notation, e.g. "2001:db8::/64"
type Update struct {
	token   string
	updater Updater
}

func New(token string, updater Updater) *Update {
	return &Update{token: token, updater: updater}
}

func (h *Update) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	if subtle.ConstantTimeCompare([]byte(q.Get("token")), []byte(h.token)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	prefix := q.Get("ip6lanprefix")
	if prefix == "" {
		http.Error(w, "missing ip6lanprefix", http.StatusBadRequest)
		return
	}

	if _, _, err := net.ParseCIDR(prefix); err != nil {
		http.Error(w, fmt.Sprintf("invalid ip6lanprefix: %v", err), http.StatusBadRequest)
		return
	}

	if err := h.updater.Update(r.Context(), prefix); err != nil {
		slog.Error("failed to update DNS records", "error", err, "prefix", prefix)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, "OK")
}
