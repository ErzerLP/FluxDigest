package api

import (
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestRouterExposesHealthz(t *testing.T) {
    router := NewRouter()
    req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
    rec := httptest.NewRecorder()

    router.ServeHTTP(rec, req)

    if rec.Code != http.StatusOK {
        t.Fatalf("want 200 got %d", rec.Code)
    }
}