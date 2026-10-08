package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
)

func TestTrustedClientChain(t *testing.T) {
	for _, c := range []struct{ peer, forward, want string }{
		{"192.0.2.1:1", "198.51.100.1", "192.0.2.1"},
		{"127.0.0.1:1", "198.51.100.1, 100.64.0.1", "100.64.0.1"},
		{"127.0.0.1:1", "198.51.100.1, garbage", "127.0.0.1"},
		{"[::1]:1", "100.64.0.1, 127.0.0.1", "100.64.0.1"},
	} {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = c.peer
		r.Header.Set("X-Forwarded-For", c.forward)
		if got := clientIP(r); got != c.want {
			t.Errorf("%+v: %s", c, got)
		}
	}
}
func TestOriginValidation(t *testing.T) {
	for _, raw := range []string{"", "http://example.com", "https://user:pass@example.com", "https://example.com/path", "https://example.com?x=1"} {
		if _, err := ValidateOrigin(raw, "127.0.0.1:8081", false); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	for _, addr := range []string{":8081", "0.0.0.0:8081", "localhost:8081", "100.64.0.1:8081"} {
		if _, err := ValidateOrigin("https://example.com", addr, false); err == nil {
			t.Errorf("accepted %s", addr)
		}
	}
	if _, err := ValidateOrigin("http://localhost:8081", "127.0.0.1:8081", true); err != nil {
		t.Fatal(err)
	}
}
func TestPublicConcurrencyLimit(t *testing.T) {
	entered := make(chan struct{}, 32)
	release := make(chan struct{})
	origin, _ := url.Parse("https://example.com")
	h := Boundary(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { entered <- struct{}{}; <-release }), origin, origin.String(), false)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "https://example.com/", nil))
		}()
	}
	for i := 0; i < 32; i++ {
		<-entered
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "https://example.com/", nil))
	if w.Code != 503 {
		t.Errorf("saturated: %d", w.Code)
	}
	close(release)
	wg.Wait()
}

func TestHashCapacityNoQueue(t *testing.T) {
	first, ok := acquireHashSlot()
	if !ok {
		t.Fatal("first slot")
	}
	defer first()
	second, ok := acquireHashSlot()
	if !ok {
		t.Fatal("second slot")
	}
	if release, ok := acquireHashSlot(); ok {
		release()
		t.Fatal("third hash admitted")
	}
	second()
	next, ok := acquireHashSlot()
	if !ok {
		t.Fatal("slot not released")
	}
	next()
}
