package hscontrol

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func TestBrandHandler(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/brand/{file}", BrandHandler)

	for _, tt := range []struct {
		path string
		code int
	}{
		{"/brand/elroi-on-light.svg", http.StatusOK},
		{"/brand/qavats-it-on-dark.svg", http.StatusOK},
		{"/brand/missing.svg", http.StatusNotFound},
		{"/brand/..%2Fstyle.css", http.StatusNotFound},
	} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.path, nil))
		assert.Equal(t, tt.code, rec.Code, tt.path)

		if tt.code == http.StatusOK {
			assert.Equal(t, "image/svg+xml", rec.Header().Get("Content-Type"))
		}
	}
}
