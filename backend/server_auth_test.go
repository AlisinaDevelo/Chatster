package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AliSinaDevelo/Chatster/internal/auth"
	"github.com/gorilla/websocket"
)

const integrationAuthUsers = `[
  {
    "token": "alice-integration-token-with-32-bytes",
    "user_id": "usr_alice",
    "display_name": "Alice",
    "rooms": ["general", "engineering"]
  }
]`

func TestSessionModeProtectsHistoryAndEnforcesRoomGrants(t *testing.T) {
	cfg, database, hub, cleanup := testStack(t)
	defer cleanup()
	authService := testAuthService(t, time.Now)

	srv := httptest.NewServer(mountWithAuth(cfg, hub, database, authService))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/messages?room=engineering")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		_ = resp.Body.Close()
		t.Fatalf("history without session: got %d want 401", resp.StatusCode)
	}
	_ = resp.Body.Close()

	invalidRequest, err := http.NewRequest(http.MethodPost, srv.URL+"/api/session", nil)
	if err != nil {
		t.Fatal(err)
	}
	invalidRequest.Header.Set("Authorization", "Bearer invalid-token")
	invalidResponse, err := http.DefaultClient.Do(invalidRequest)
	if err != nil {
		t.Fatal(err)
	}
	if invalidResponse.StatusCode != http.StatusUnauthorized {
		_ = invalidResponse.Body.Close()
		t.Fatalf("invalid login: got %d want 401", invalidResponse.StatusCode)
	}
	_ = invalidResponse.Body.Close()

	cookie := loginForSession(t, srv.URL)
	authorizedRequest, err := http.NewRequest(http.MethodGet, srv.URL+"/api/messages?room=engineering", nil)
	if err != nil {
		t.Fatal(err)
	}
	authorizedRequest.AddCookie(cookie)
	authorizedResponse, err := http.DefaultClient.Do(authorizedRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = authorizedResponse.Body.Close() }()
	if authorizedResponse.StatusCode != http.StatusOK {
		t.Fatalf("authorized history: got %d want 200", authorizedResponse.StatusCode)
	}
	if cacheControl := authorizedResponse.Header.Get("Cache-Control"); cacheControl != "private, no-store" {
		t.Fatalf("authenticated history cache control: got %q", cacheControl)
	}
	var history struct {
		Viewer auth.Principal `json:"viewer"`
	}
	if err := json.NewDecoder(authorizedResponse.Body).Decode(&history); err != nil {
		t.Fatal(err)
	}
	if history.Viewer.UserID != "usr_alice" || history.Viewer.DisplayName != "Alice" {
		t.Fatalf("history viewer did not bind identity: %#v", history.Viewer)
	}

	deniedRequest, err := http.NewRequest(http.MethodGet, srv.URL+"/api/messages?room=off-topic", nil)
	if err != nil {
		t.Fatal(err)
	}
	deniedRequest.AddCookie(cookie)
	deniedResponse, err := http.DefaultClient.Do(deniedRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deniedResponse.Body.Close() }()
	if deniedResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("denied room: got %d want 403", deniedResponse.StatusCode)
	}
}

func TestSessionModeRejectsExpiredHistorySession(t *testing.T) {
	cfg, database, hub, cleanup := testStack(t)
	defer cleanup()
	now := time.Date(2026, time.August, 8, 12, 0, 0, 0, time.UTC)
	authService := testAuthService(t, func() time.Time { return now })
	_, cookie, err := authService.Exchange("alice-integration-token-with-32-bytes")
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(31 * time.Minute)

	srv := httptest.NewServer(mountWithAuth(cfg, hub, database, authService))
	defer srv.Close()
	request, err := http.NewRequest(http.MethodGet, srv.URL+"/api/messages?room=general", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.AddCookie(cookie)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expired history session: got %d want 401", response.StatusCode)
	}
	if code := responseErrorCode(t, response); code != "session_expired" {
		t.Fatalf("expired error code: got %q want session_expired", code)
	}

	headers := http.Header{}
	headers.Set("Cookie", cookie.String())
	connection, wsResponse, err := websocket.DefaultDialer.Dial(wsURLRoom(srv, "general"), headers)
	if err == nil {
		_ = connection.Close()
		t.Fatal("WebSocket with an expired session should fail")
	}
	if wsResponse == nil || wsResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expired WebSocket response: %#v", wsResponse)
	}
}

type failingRandomReader struct{}

func (failingRandomReader) Read([]byte) (int, error) {
	return 0, errors.New("entropy unavailable")
}

func TestSessionEndpointSeparatesInternalFailureFromInvalidCredentials(t *testing.T) {
	cfg, database, hub, cleanup := testStack(t)
	defer cleanup()
	authService, err := auth.New(auth.Config{
		Mode:            auth.ModeSession,
		SessionSecret:   strings.Repeat("s", 32),
		CredentialsJSON: integrationAuthUsers,
		SessionTTL:      30 * time.Minute,
		Random:          failingRandomReader{},
	})
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(mountWithAuth(cfg, hub, database, authService))
	defer srv.Close()
	request, err := http.NewRequest(http.MethodPost, srv.URL+"/api/session", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer alice-integration-token-with-32-bytes")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusInternalServerError {
		t.Fatalf("session creation failure: got %d want 500", response.StatusCode)
	}
	if code := responseErrorCode(t, response); code != "session_unavailable" {
		t.Fatalf("session creation error code: got %q want session_unavailable", code)
	}
}

func TestSessionEndpointEnforcesBrowserOriginAndSupportsLogout(t *testing.T) {
	cfg, database, hub, cleanup := testStack(t)
	defer cleanup()
	cfg.AllowedOrigins = []string{"https://app.example"}
	authService := testAuthService(t, time.Now)

	srv := httptest.NewServer(mountWithAuth(cfg, hub, database, authService))
	defer srv.Close()

	deniedRequest, err := http.NewRequest(http.MethodPost, srv.URL+"/api/session", nil)
	if err != nil {
		t.Fatal(err)
	}
	deniedRequest.Header.Set("Origin", "https://evil.example")
	deniedRequest.Header.Set("Authorization", "Bearer alice-integration-token-with-32-bytes")
	deniedResponse, err := http.DefaultClient.Do(deniedRequest)
	if err != nil {
		t.Fatal(err)
	}
	if deniedResponse.StatusCode != http.StatusForbidden {
		_ = deniedResponse.Body.Close()
		t.Fatalf("denied login origin: got %d want 403", deniedResponse.StatusCode)
	}
	_ = deniedResponse.Body.Close()

	loginRequest, err := http.NewRequest(http.MethodPost, srv.URL+"/api/session", nil)
	if err != nil {
		t.Fatal(err)
	}
	loginRequest.Header.Set("Origin", "https://app.example")
	loginRequest.Header.Set("Authorization", "Bearer alice-integration-token-with-32-bytes")
	loginResponse, err := http.DefaultClient.Do(loginRequest)
	if err != nil {
		t.Fatal(err)
	}
	if loginResponse.StatusCode != http.StatusOK {
		_ = loginResponse.Body.Close()
		t.Fatalf("allowed login origin: got %d want 200", loginResponse.StatusCode)
	}
	if loginResponse.Header.Get("Access-Control-Allow-Origin") != "https://app.example" || loginResponse.Header.Get("Access-Control-Allow-Credentials") != "true" {
		_ = loginResponse.Body.Close()
		t.Fatalf("credentialed CORS headers: %#v", loginResponse.Header)
	}
	var sessionCookie *http.Cookie
	for _, cookie := range loginResponse.Cookies() {
		if cookie.Name == auth.CookieName {
			sessionCookie = cookie
			break
		}
	}
	_ = loginResponse.Body.Close()
	if sessionCookie == nil {
		t.Fatal("allowed login did not return a session cookie")
	}

	stateRequest, err := http.NewRequest(http.MethodGet, srv.URL+"/api/session", nil)
	if err != nil {
		t.Fatal(err)
	}
	stateRequest.Header.Set("Origin", "https://app.example")
	stateRequest.AddCookie(sessionCookie)
	stateResponse, err := http.DefaultClient.Do(stateRequest)
	if err != nil {
		t.Fatal(err)
	}
	var state struct {
		Authenticated bool           `json:"authenticated"`
		User          auth.Principal `json:"user"`
	}
	if err := json.NewDecoder(stateResponse.Body).Decode(&state); err != nil {
		_ = stateResponse.Body.Close()
		t.Fatal(err)
	}
	_ = stateResponse.Body.Close()
	if !state.Authenticated || state.User.UserID != "usr_alice" {
		t.Fatalf("session state: %#v", state)
	}

	logoutRequest, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/session", nil)
	if err != nil {
		t.Fatal(err)
	}
	logoutRequest.Header.Set("Origin", "https://app.example")
	logoutRequest.AddCookie(sessionCookie)
	logoutResponse, err := http.DefaultClient.Do(logoutRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = logoutResponse.Body.Close() }()
	if logoutResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("logout: got %d want 204", logoutResponse.StatusCode)
	}
	if cookies := logoutResponse.Cookies(); len(cookies) != 1 || cookies[0].Name != auth.CookieName || cookies[0].MaxAge != -1 {
		t.Fatalf("logout cookie: %#v", cookies)
	}
}

func TestBearerTokenRejectsOversizeInput(t *testing.T) {
	if _, ok := bearerToken("Bearer " + strings.Repeat("t", auth.MaxAccessTokenBytes+1)); ok {
		t.Fatal("oversize bearer token should be rejected")
	}
}

func TestSessionModeBindsWebSocketIdentityAndRejectsDeniedRoom(t *testing.T) {
	cfg, database, hub, cleanup := testStack(t)
	defer cleanup()
	authService := testAuthService(t, time.Now)
	_, cookie, err := authService.Exchange("alice-integration-token-with-32-bytes")
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(mountWithAuth(cfg, hub, database, authService))
	defer srv.Close()

	unauthorized, response, err := websocket.DefaultDialer.Dial(wsURLRoom(srv, "general"), nil)
	if err == nil {
		_ = unauthorized.Close()
		t.Fatal("WebSocket without a session should fail")
	}
	if response == nil || response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthorized WebSocket response: %#v", response)
	}

	headers := http.Header{}
	headers.Set("Cookie", cookie.String())
	denied, response, err := websocket.DefaultDialer.Dial(wsURLRoom(srv, "off-topic"), headers)
	if err == nil {
		_ = denied.Close()
		t.Fatal("WebSocket for a denied room should fail")
	}
	if response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("denied WebSocket response: %#v", response)
	}

	connection, response, err := websocket.DefaultDialer.Dial(wsURLRoom(srv, "engineering"), headers)
	if err != nil {
		t.Fatalf("authorized WebSocket dial: %v (response=%v)", err, response)
	}
	defer func() { _ = connection.Close() }()
	if err := connection.WriteJSON(Message{Type: "username", Content: "Mallory"}); err != nil {
		t.Fatal(err)
	}
	if err := connection.WriteJSON(Message{Type: "message", Content: "identity bound"}); err != nil {
		t.Fatal(err)
	}

	message := readMatchingMessage(t, connection, func(message Message) bool {
		return message.Type == "message" && message.Content == "identity bound"
	})
	if message.UserID != "usr_alice" || message.Username != "Alice" {
		t.Fatalf("WebSocket message identity: %#v", message)
	}

	var storedUserID, storedUsername string
	if err := database.QueryRow(
		"SELECT user_id, username FROM messages WHERE content = ?",
		"identity bound",
	).Scan(&storedUserID, &storedUsername); err != nil {
		t.Fatal(err)
	}
	if storedUserID != "usr_alice" || storedUsername != "Alice" {
		t.Fatalf("stored identity: user_id=%q username=%q", storedUserID, storedUsername)
	}

	event := latestModerationEvent(t, database)
	if event.Reason != "identity_override" || event.UserID != "usr_alice" {
		t.Fatalf("identity override audit: %#v", event)
	}
}

func TestAnonymousModeRemainsExplicitlyAvailable(t *testing.T) {
	cfg, database, hub, cleanup := testStack(t)
	defer cleanup()
	authService, err := auth.New(auth.Config{Mode: auth.ModeAnonymous})
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(mountWithAuth(cfg, hub, database, authService))
	defer srv.Close()
	response, err := http.Get(srv.URL + "/api/messages?room=off-topic")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("anonymous history: got %d want 200", response.StatusCode)
	}

	connection, wsResponse, err := websocket.DefaultDialer.Dial(wsURLRoom(srv, "off-topic"), nil)
	if err != nil {
		t.Fatalf("anonymous WebSocket: %v (response=%v)", err, wsResponse)
	}
	_ = connection.Close()
}

func testAuthService(t *testing.T, now func() time.Time) *auth.Service {
	t.Helper()
	service, err := auth.New(auth.Config{
		Mode:            auth.ModeSession,
		SessionSecret:   strings.Repeat("s", 32),
		CredentialsJSON: integrationAuthUsers,
		SessionTTL:      30 * time.Minute,
		CookieSecure:    false,
		Now:             now,
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func loginForSession(t *testing.T, baseURL string) *http.Cookie {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, baseURL+"/api/session", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer alice-integration-token-with-32-bytes")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("login status: got %d want 200", response.StatusCode)
	}
	for _, cookie := range response.Cookies() {
		if cookie.Name == auth.CookieName {
			return cookie
		}
	}
	t.Fatal("login response did not set the session cookie")
	return nil
}

func responseErrorCode(t *testing.T, response *http.Response) string {
	t.Helper()
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil && !errors.Is(err, http.ErrBodyReadAfterClose) {
		t.Fatal(err)
	}
	return payload.Error.Code
}
