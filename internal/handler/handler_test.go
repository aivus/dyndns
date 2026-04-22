package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type mockUpdater struct {
	err error
}

func (m *mockUpdater) Update(_ context.Context, _, _ string) error {
	return m.err
}

func TestUpdate_ValidRequest(t *testing.T) {
	h := New("secret", &mockUpdater{})
	r := httptest.NewRequest(http.MethodGet, "/update?token=secret&ip6lanprefix=2001:db8::/64", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if body := w.Body.String(); body != "OK" {
		t.Errorf("body = %q, want %q", body, "OK")
	}
}

func TestUpdate_WrongToken(t *testing.T) {
	h := New("secret", &mockUpdater{})
	r := httptest.NewRequest(http.MethodGet, "/update?token=wrong&ip6lanprefix=2001:db8::/64", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	if !strings.Contains(w.Body.String(), "unauthorized") {
		t.Errorf("body %q does not contain %q", w.Body.String(), "unauthorized")
	}
}

func TestUpdate_MissingBothParams(t *testing.T) {
	h := New("secret", &mockUpdater{})
	r := httptest.NewRequest(http.MethodGet, "/update?token=secret", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if !strings.Contains(w.Body.String(), "missing") {
		t.Errorf("body %q does not contain %q", w.Body.String(), "missing")
	}
}

func TestUpdate_ValidIp6addr(t *testing.T) {
	h := New("secret", &mockUpdater{})
	r := httptest.NewRequest(http.MethodGet, "/update?token=secret&ip6addr=2001:db8::1", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestUpdate_MalformedIp6addr(t *testing.T) {
	h := New("secret", &mockUpdater{})
	r := httptest.NewRequest(http.MethodGet, "/update?token=secret&ip6addr=notanip", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if !strings.Contains(w.Body.String(), "invalid") {
		t.Errorf("body %q does not contain %q", w.Body.String(), "invalid")
	}
}

func TestUpdate_MalformedPrefix(t *testing.T) {
	h := New("secret", &mockUpdater{})
	r := httptest.NewRequest(http.MethodGet, "/update?token=secret&ip6lanprefix=notaprefix", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if !strings.Contains(w.Body.String(), "invalid") {
		t.Errorf("body %q does not contain %q", w.Body.String(), "invalid")
	}
}

func TestUpdate_UpdaterError(t *testing.T) {
	h := New("secret", &mockUpdater{err: errors.New("cloudflare timeout")})
	r := httptest.NewRequest(http.MethodGet, "/update?token=secret&ip6lanprefix=2001:db8::/64", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(w.Body.String(), "internal error") {
		t.Errorf("body %q does not contain %q", w.Body.String(), "internal error")
	}
}
