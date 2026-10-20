package isbn

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Covers only come from public HTTPS servers: not over plain HTTP, and not
// from addresses inside the network, which a cover URL could point at.
func TestFetchCoverRefusesLocalAddresses(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("secret"))
	}))
	defer srv.Close()
	for _, u := range []string{srv.URL + "/cover.jpg", "https://127.0.0.1/", "https://192.168.1.1/", "https://[::1]/", "http://example.com/a.jpg"} {
		if _, err := FetchCover(context.Background(), u); err == nil {
			t.Errorf("%s: fetched", u)
		} else if strings.Contains(err.Error(), "secret") {
			t.Errorf("%s: %v", u, err)
		}
	}
}
