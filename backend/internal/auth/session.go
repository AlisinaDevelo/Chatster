// Package auth owns Chatster's optional signed-session trust boundary.
package auth

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/AliSinaDevelo/Chatster/db"
)

const (
	// ModeAnonymous preserves the public demo's unauthenticated behavior.
	ModeAnonymous = "anonymous"
	// ModeSession requires a valid signed cookie and room grant.
	ModeSession = "session"
	// CookieName is intentionally fixed so frontend code never handles the value.
	CookieName = "chatster_session"

	defaultSessionTTL = time.Hour
	minSessionTTL     = time.Minute
	maxSessionTTL     = 24 * time.Hour
	minSecretBytes    = 32
	minTokenBytes     = 32
	maxRoomsPerUser   = 64
	maxCookieBytes    = 4096
)

// MaxAccessTokenBytes bounds credential configuration and login request work.
const MaxAccessTokenBytes = 512

var (
	// ErrAuthDisabled indicates that the server is running in anonymous mode.
	ErrAuthDisabled = errors.New("session authentication is disabled")
	// ErrInvalidToken indicates that an access token is not configured or valid.
	ErrInvalidToken = errors.New("invalid access token")
	// ErrSessionRequired indicates that a request has no session cookie.
	ErrSessionRequired = errors.New("session required")
	// ErrInvalidSession indicates that a session cookie cannot be verified.
	ErrInvalidSession = errors.New("invalid session")
	// ErrExpiredSession indicates that a verified session is past its expiry.
	ErrExpiredSession = errors.New("session expired")
	// ErrRoomDenied indicates that the principal has no grant for the room.
	ErrRoomDenied = errors.New("room access denied")
)

// Config contains only server-side authentication configuration.
type Config struct {
	Mode            string
	SessionSecret   string
	CredentialsJSON string
	SessionTTL      time.Duration
	CookieSecure    bool
	Now             func() time.Time
	Random          io.Reader
}

type credential struct {
	Token       string   `json:"token"`
	UserID      string   `json:"user_id"`
	DisplayName string   `json:"display_name"`
	Rooms       []string `json:"rooms"`
}

type principalTemplate struct {
	UserID      string
	DisplayName string
	Rooms       []string
}

// Principal is the identity and authorization state bound to a request.
type Principal struct {
	UserID      string    `json:"user_id,omitempty"`
	DisplayName string    `json:"display_name"`
	Rooms       []string  `json:"rooms,omitempty"`
	Anonymous   bool      `json:"anonymous"`
	SessionID   string    `json:"-"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
}

// Authenticated reports whether the principal came from a verified session.
func (p Principal) Authenticated() bool {
	return !p.Anonymous && p.UserID != "" && p.SessionID != ""
}

type sessionPayload struct {
	Version     int      `json:"v"`
	SessionID   string   `json:"sid"`
	Subject     string   `json:"sub"`
	DisplayName string   `json:"name"`
	Rooms       []string `json:"rooms"`
	IssuedAt    int64    `json:"iat"`
	ExpiresAt   int64    `json:"exp"`
}

// Service exchanges operator-provisioned tokens and validates signed sessions.
type Service struct {
	mode         string
	secret       []byte
	credentials  map[[sha256.Size]byte]principalTemplate
	ttl          time.Duration
	cookieSecure bool
	now          func() time.Time
	random       io.Reader
}

// New validates all security-sensitive configuration before serving traffic.
func New(cfg Config) (*Service, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		mode = ModeAnonymous
	}

	service := &Service{
		mode:         mode,
		cookieSecure: cfg.CookieSecure,
		now:          cfg.Now,
		random:       cfg.Random,
	}
	if service.now == nil {
		service.now = time.Now
	}
	if service.random == nil {
		service.random = rand.Reader
	}

	switch mode {
	case ModeAnonymous:
		return service, nil
	case ModeSession:
		// Continue with strict session configuration validation below.
	default:
		return nil, fmt.Errorf("unsupported auth mode %q; use anonymous or session", mode)
	}

	if len([]byte(cfg.SessionSecret)) < minSecretBytes {
		return nil, fmt.Errorf("session secret must be at least %d bytes", minSecretBytes)
	}
	service.secret = []byte(cfg.SessionSecret)
	service.ttl = cfg.SessionTTL
	if service.ttl == 0 {
		service.ttl = defaultSessionTTL
	}
	if service.ttl < minSessionTTL || service.ttl > maxSessionTTL {
		return nil, fmt.Errorf("session TTL must be between %s and %s", minSessionTTL, maxSessionTTL)
	}

	credentials, err := decodeCredentials(cfg.CredentialsJSON)
	if err != nil {
		return nil, err
	}
	service.credentials = make(map[[sha256.Size]byte]principalTemplate, len(credentials))
	for _, configured := range credentials {
		tokenBytes := len([]byte(configured.Token))
		if tokenBytes < minTokenBytes || tokenBytes > MaxAccessTokenBytes {
			return nil, fmt.Errorf("each access token must be between %d and %d bytes", minTokenBytes, MaxAccessTokenBytes)
		}
		if strings.TrimSpace(configured.Token) != configured.Token || strings.ContainsAny(configured.Token, " \t\r\n") {
			return nil, errors.New("access tokens cannot contain whitespace")
		}
		template, err := normalizePrincipal(configured.UserID, configured.DisplayName, configured.Rooms)
		if err != nil {
			return nil, err
		}
		tokenHash := sha256.Sum256([]byte(configured.Token))
		if _, exists := service.credentials[tokenHash]; exists {
			return nil, errors.New("duplicate access token")
		}
		service.credentials[tokenHash] = template
	}

	return service, nil
}

// Mode returns the configured authentication mode.
func (s *Service) Mode() string {
	if s == nil || s.mode == "" {
		return ModeAnonymous
	}
	return s.mode
}

// Exchange validates an opaque access token and returns a signed session cookie.
func (s *Service) Exchange(token string) (Principal, *http.Cookie, error) {
	if s == nil || s.Mode() != ModeSession {
		return Principal{}, nil, ErrAuthDisabled
	}
	if size := len([]byte(token)); size < minTokenBytes || size > MaxAccessTokenBytes || strings.ContainsAny(token, " \t\r\n") {
		return Principal{}, nil, ErrInvalidToken
	}
	tokenHash := sha256.Sum256([]byte(token))
	template, ok := s.credentials[tokenHash]
	if !ok {
		return Principal{}, nil, ErrInvalidToken
	}

	sessionID, err := s.newSessionID()
	if err != nil {
		return Principal{}, nil, fmt.Errorf("create session id: %w", err)
	}
	now := s.now().UTC().Truncate(time.Second)
	expiresAt := now.Add(s.ttl)
	payload := sessionPayload{
		Version:     1,
		SessionID:   sessionID,
		Subject:     template.UserID,
		DisplayName: template.DisplayName,
		Rooms:       append([]string(nil), template.Rooms...),
		IssuedAt:    now.Unix(),
		ExpiresAt:   expiresAt.Unix(),
	}
	value, err := s.encode(payload)
	if err != nil {
		return Principal{}, nil, err
	}

	principal := Principal{
		UserID:      payload.Subject,
		DisplayName: payload.DisplayName,
		Rooms:       append([]string(nil), payload.Rooms...),
		SessionID:   payload.SessionID,
		ExpiresAt:   expiresAt,
	}
	cookie := &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(s.ttl / time.Second),
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteStrictMode,
	}
	return principal, cookie, nil
}

// Authenticate validates the request cookie and returns its stable principal.
func (s *Service) Authenticate(r *http.Request) (Principal, error) {
	if s == nil || s.Mode() == ModeAnonymous {
		return Principal{DisplayName: "Anonymous", Anonymous: true}, nil
	}
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return Principal{}, ErrSessionRequired
		}
		return Principal{}, ErrInvalidSession
	}
	if cookie.Value == "" || len(cookie.Value) > maxCookieBytes {
		return Principal{}, ErrInvalidSession
	}

	payload, err := s.decode(cookie.Value)
	if err != nil {
		return Principal{}, err
	}
	template, err := normalizePrincipal(payload.Subject, payload.DisplayName, payload.Rooms)
	if err != nil || payload.Version != 1 || !validOpaqueID(payload.SessionID, "sid_") {
		return Principal{}, ErrInvalidSession
	}
	issuedAt := time.Unix(payload.IssuedAt, 0).UTC()
	expiresAt := time.Unix(payload.ExpiresAt, 0).UTC()
	now := s.now().UTC()
	if !expiresAt.After(now) {
		return Principal{}, ErrExpiredSession
	}
	if payload.IssuedAt <= 0 || !expiresAt.After(issuedAt) || issuedAt.After(now.Add(time.Minute)) {
		return Principal{}, ErrInvalidSession
	}
	if expiresAt.Sub(issuedAt) > s.ttl+time.Second {
		return Principal{}, ErrInvalidSession
	}

	return Principal{
		UserID:      template.UserID,
		DisplayName: template.DisplayName,
		Rooms:       append([]string(nil), template.Rooms...),
		SessionID:   payload.SessionID,
		ExpiresAt:   expiresAt,
	}, nil
}

// Authorize validates identity and checks the requested room grant.
func (s *Service) Authorize(r *http.Request, room string) (Principal, error) {
	principal, err := s.Authenticate(r)
	if err != nil {
		return Principal{}, err
	}
	room, err = db.NormalizeRoom(room)
	if err != nil {
		return Principal{}, ErrRoomDenied
	}
	if principal.Anonymous {
		return principal, nil
	}
	for _, allowed := range principal.Rooms {
		if allowed == room {
			return principal, nil
		}
	}
	return Principal{}, ErrRoomDenied
}

// ExpiredCookie removes the browser's local session cookie.
func (s *Service) ExpiredCookie() *http.Cookie {
	secure := false
	if s != nil {
		secure = s.cookieSecure
	}
	return &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(1, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	}
}

func (s *Service) newSessionID() (string, error) {
	var raw [16]byte
	if _, err := io.ReadFull(s.random, raw[:]); err != nil {
		return "", err
	}
	return "sid_" + hex.EncodeToString(raw[:]), nil
}

func (s *Service) encode(payload sessionPayload) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode session: %w", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(encoded))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	value := encoded + "." + signature
	if len(value) > maxCookieBytes {
		return "", errors.New("encoded session exceeds cookie limit")
	}
	return value, nil
}

func (s *Service) decode(value string) (sessionPayload, error) {
	encoded, signature, ok := strings.Cut(value, ".")
	if !ok || encoded == "" || signature == "" || strings.Contains(signature, ".") {
		return sessionPayload{}, ErrInvalidSession
	}
	providedMAC, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return sessionPayload{}, ErrInvalidSession
	}
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(encoded))
	if !hmac.Equal(providedMAC, mac.Sum(nil)) {
		return sessionPayload{}, ErrInvalidSession
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return sessionPayload{}, ErrInvalidSession
	}
	var payload sessionPayload
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return sessionPayload{}, ErrInvalidSession
	}
	if err := requireJSONEOF(decoder); err != nil {
		return sessionPayload{}, ErrInvalidSession
	}
	return payload, nil
}

func decodeCredentials(raw string) ([]credential, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("session mode requires CHATSTER_AUTH_USERS_JSON")
	}
	var credentials []credential
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&credentials); err != nil {
		return nil, fmt.Errorf("parse auth users JSON: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return nil, errors.New("parse auth users JSON: trailing data")
	}
	if len(credentials) == 0 {
		return nil, errors.New("session mode requires at least one auth user")
	}
	return credentials, nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("unexpected trailing JSON")
		}
		return err
	}
	return nil
}

func normalizePrincipal(userID, displayName string, rooms []string) (principalTemplate, error) {
	userID = strings.TrimSpace(userID)
	if !validIdentifier(userID) {
		return principalTemplate{}, errors.New("auth user_id must be 1-128 ASCII letters, digits, dot, underscore, colon, or hyphen")
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" || utf8.RuneCountInString(displayName) > 64 {
		return principalTemplate{}, errors.New("auth display_name must be 1-64 characters")
	}
	if len(rooms) == 0 || len(rooms) > maxRoomsPerUser {
		return principalTemplate{}, fmt.Errorf("auth user must have between 1 and %d rooms", maxRoomsPerUser)
	}
	normalizedRooms := make([]string, 0, len(rooms))
	seen := make(map[string]struct{}, len(rooms))
	for _, room := range rooms {
		if strings.TrimSpace(room) == "" {
			return principalTemplate{}, errors.New("auth user room cannot be empty")
		}
		normalized, err := db.NormalizeRoom(room)
		if err != nil {
			return principalTemplate{}, fmt.Errorf("auth user room: %w", err)
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		normalizedRooms = append(normalizedRooms, normalized)
	}
	return principalTemplate{UserID: userID, DisplayName: displayName, Rooms: normalizedRooms}, nil
}

func validIdentifier(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, char := range value {
		isLetter := char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z'
		isDigit := char >= '0' && char <= '9'
		if !isLetter && !isDigit && char != '.' && char != '_' && char != ':' && char != '-' {
			return false
		}
	}
	return true
}

func validOpaqueID(value, prefix string) bool {
	if !strings.HasPrefix(value, prefix) || len(value) != len(prefix)+32 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, prefix))
	return err == nil
}
