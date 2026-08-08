package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testCredentials = `[
  {
    "token": "alice-access-token-with-32-bytes-minimum",
    "user_id": "usr_alice",
    "display_name": "Alice",
    "rooms": ["general", "engineering"]
  }
]`

func TestAnonymousModeAllowsRoomWithoutSession(t *testing.T) {
	service, err := New(Config{Mode: ModeAnonymous})
	if err != nil {
		t.Fatal(err)
	}

	principal, err := service.Authorize(httptest.NewRequest("GET", "/api/messages", nil), "off-topic")
	if err != nil {
		t.Fatalf("authorize anonymous room: %v", err)
	}
	if !principal.Anonymous || principal.Authenticated() {
		t.Fatalf("unexpected anonymous principal: %#v", principal)
	}
}

func TestSessionExchangeBindsIdentityAndRoomGrants(t *testing.T) {
	now := time.Date(2026, time.August, 8, 12, 0, 0, 0, time.UTC)
	service := newTestService(t, func() time.Time { return now })

	principal, cookie, err := service.Exchange("alice-access-token-with-32-bytes-minimum")
	if err != nil {
		t.Fatalf("exchange token: %v", err)
	}
	if principal.UserID != "usr_alice" || principal.DisplayName != "Alice" || !principal.Authenticated() {
		t.Fatalf("unexpected principal: %#v", principal)
	}
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Value == "" {
		t.Fatalf("session cookie missing security attributes: %#v", cookie)
	}

	request := httptest.NewRequest("GET", "/api/messages?room=engineering", nil)
	request.AddCookie(cookie)
	authorized, err := service.Authorize(request, "engineering")
	if err != nil {
		t.Fatalf("authorize granted room: %v", err)
	}
	if authorized.UserID != "usr_alice" || authorized.SessionID == "" {
		t.Fatalf("session did not preserve identity: %#v", authorized)
	}

	if _, err := service.Authorize(request, "off-topic"); !errors.Is(err, ErrRoomDenied) {
		t.Fatalf("denied room: got %v want %v", err, ErrRoomDenied)
	}
}

func TestSessionRejectsExpiredAndTamperedCookies(t *testing.T) {
	now := time.Date(2026, time.August, 8, 12, 0, 0, 0, time.UTC)
	service := newTestService(t, func() time.Time { return now })
	_, cookie, err := service.Exchange("alice-access-token-with-32-bytes-minimum")
	if err != nil {
		t.Fatal(err)
	}

	tampered := *cookie
	replacement := "x"
	if strings.HasSuffix(cookie.Value, replacement) {
		replacement = "y"
	}
	tampered.Value = cookie.Value[:len(cookie.Value)-1] + replacement
	tamperedRequest := httptest.NewRequest("GET", "/api/messages", nil)
	tamperedRequest.AddCookie(&tampered)
	if _, err := service.Authenticate(tamperedRequest); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("tampered cookie: got %v want %v", err, ErrInvalidSession)
	}

	now = now.Add(31 * time.Minute)
	expiredRequest := httptest.NewRequest("GET", "/api/messages", nil)
	expiredRequest.AddCookie(cookie)
	if _, err := service.Authenticate(expiredRequest); !errors.Is(err, ErrExpiredSession) {
		t.Fatalf("expired cookie: got %v want %v", err, ErrExpiredSession)
	}
}

func TestSessionModeRejectsUnsafeConfiguration(t *testing.T) {
	_, err := New(Config{
		Mode:            ModeSession,
		SessionSecret:   "short",
		CredentialsJSON: testCredentials,
		SessionTTL:      30 * time.Minute,
	})
	if err == nil || !strings.Contains(err.Error(), "session secret") {
		t.Fatalf("short secret: got %v", err)
	}
}

func TestSessionModeRejectsTokenWhitespace(t *testing.T) {
	credentials := strings.Replace(testCredentials, "alice-access-token", "alice access-token", 1)
	_, err := New(Config{
		Mode:            ModeSession,
		SessionSecret:   strings.Repeat("s", 32),
		CredentialsJSON: credentials,
		SessionTTL:      30 * time.Minute,
	})
	if err == nil || !strings.Contains(err.Error(), "whitespace") {
		t.Fatalf("token whitespace: got %v", err)
	}
}

func TestSessionModeBoundsCredentialAndRoomGrantSizes(t *testing.T) {
	tests := []struct {
		name        string
		credentials string
		want        string
	}{
		{
			name:        "token too long",
			credentials: strings.Replace(testCredentials, "alice-access-token-with-32-bytes-minimum", strings.Repeat("t", MaxAccessTokenBytes+1), 1),
			want:        "between",
		},
		{
			name:        "too many rooms",
			credentials: strings.Replace(testCredentials, `"general", "engineering"`, strings.Repeat(`"general",`, maxRoomsPerUser)+`"general"`, 1),
			want:        "rooms",
		},
		{
			name:        "empty room",
			credentials: strings.Replace(testCredentials, `"general", "engineering"`, `""`, 1),
			want:        "cannot be empty",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New(Config{
				Mode:            ModeSession,
				SessionSecret:   strings.Repeat("s", 32),
				CredentialsJSON: test.credentials,
				SessionTTL:      30 * time.Minute,
			})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("unsafe credentials: got %v, want error containing %q", err, test.want)
			}
		})
	}
}

func TestSessionExchangeRejectsOversizeToken(t *testing.T) {
	service := newTestService(t, time.Now)
	if _, _, err := service.Exchange(strings.Repeat("t", MaxAccessTokenBytes+1)); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("oversize token: got %v want %v", err, ErrInvalidToken)
	}
}

func newTestService(t *testing.T, now func() time.Time) *Service {
	t.Helper()
	service, err := New(Config{
		Mode:            ModeSession,
		SessionSecret:   strings.Repeat("s", 32),
		CredentialsJSON: testCredentials,
		SessionTTL:      30 * time.Minute,
		CookieSecure:    false,
		Now:             now,
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}
