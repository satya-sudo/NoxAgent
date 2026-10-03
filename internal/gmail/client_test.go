package gmail

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestAuthorizationURLCreatesOneTimeState(t *testing.T) {
	client := testClient(t)
	rawURL, err := client.AuthorizationURL()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	state := parsed.Query().Get("state")
	if state == "" {
		t.Fatal("authorization URL has no state")
	}
	if !client.consumeState(state) {
		t.Fatal("fresh state was rejected")
	}
	if client.consumeState(state) {
		t.Fatal("OAuth state was accepted twice")
	}
}

func TestCompleteAuthorizationSavesProtectedToken(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("code") != "test-code" {
			t.Fatalf("code = %q", r.Form.Get("code"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"access","refresh_token":"refresh","token_type":"Bearer","expires_in":3600}`))
	}))
	defer tokenServer.Close()

	client := testClient(t)
	client.oauth.Endpoint.TokenURL = tokenServer.URL
	client.states["valid-state"] = time.Now().Add(time.Minute)
	if err := client.CompleteAuthorization(context.Background(), "valid-state", "test-code"); err != nil {
		t.Fatal(err)
	}
	token, err := loadToken(client.tokenPath)
	if err != nil {
		t.Fatal(err)
	}
	if token.RefreshToken != "refresh" {
		t.Fatalf("refresh token = %q", token.RefreshToken)
	}
	info, err := os.Stat(client.tokenPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("token permissions = %o", info.Mode().Perm())
	}
}

func TestCompleteAuthorizationRejectsUnknownState(t *testing.T) {
	client := testClient(t)
	if err := client.CompleteAuthorization(context.Background(), "unknown", "code"); err != ErrInvalidState {
		t.Fatalf("error = %v", err)
	}
}

func testClient(t *testing.T) *Client {
	t.Helper()
	return &Client{
		oauth: &oauth2.Config{
			ClientID:     "client-id",
			ClientSecret: "client-secret",
			RedirectURL:  "http://127.0.0.1/callback",
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://accounts.example/authorize",
				TokenURL: "https://accounts.example/token",
			},
		},
		tokenPath: filepath.Join(t.TempDir(), "config", "gmail-token.json"),
		now:       time.Now,
		states:    make(map[string]time.Time),
	}
}
