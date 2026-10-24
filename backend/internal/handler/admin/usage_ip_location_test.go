package admin

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type locationRoundTripper func(*http.Request) (*http.Response, error)

func (f locationRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestUsageIPLocation(t *testing.T) {
	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()
	calls := 0
	http.DefaultTransport = locationRoundTripper(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != "https://ipwho.is/8.8.8.8" {
			t.Fatalf("unexpected lookup: %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"success":true,"country":"United States","region":"California","city":"Mountain View","connection":{"isp":"Google"}}`)), Header: make(http.Header)}, nil
	})
	router := gin.New()
	router.GET("/location", (&UsageHandler{}).IPLocation)
	for _, ip := range []string{"127.0.0.1", "10.0.0.1", "::1", "::ffff:192.168.1.1", "example.com", "8.8.8.8"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/location?ip="+ip, nil))
		if ip == "8.8.8.8" {
			if w.Code != 200 || !strings.Contains(w.Body.String(), `"isp":"Google"`) {
				t.Fatalf("lookup response: %d %s", w.Code, w.Body)
			}
		} else if w.Code != 400 {
			t.Fatalf("invalid IP %s accepted: %d", ip, w.Code)
		}
	}
	if calls != 1 {
		t.Fatalf("unexpected upstream calls: %d", calls)
	}
}
