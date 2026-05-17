package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
)

// consentBinding is the payload signed by OAUTH_COOKIE_SECRET and rendered
// into the authorize form as a hidden field. The POST handler verifies the
// signature and compares the bound fields against the POSTed values — any
// mismatch (or signature break) means CSRF / tampering, and the POST is
// rejected. The binding is stateless so no session row is required.
type consentBinding struct {
	ClientID    string
	RedirectURI string
	Challenge   string
	State       string
	IssuedAt    int64 // unix seconds
}

// consentTTL bounds how long a rendered authorize form is acceptable for
// POST-back. 10 minutes is generous for a human-completed form and short
// enough that a stolen form has limited replay value.
const consentTTL = 10 * time.Minute

// consentSep is U+001F (Unit Separator). OAuth parameter values are
// restricted to printable ASCII (RFC 6749 §3.1, §3.3), so embedding a
// non-printable separator in the signed payload cannot collide with a
// legitimate field value.
const consentSep = "\x1f"

func signConsent(secret []byte, b consentBinding) string {
	payload := strings.Join([]string{
		b.ClientID, b.RedirectURI, b.Challenge, b.State,
		strconv.FormatInt(b.IssuedAt, 10),
	}, consentSep)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." +
		base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func verifyConsent(secret []byte, token string, now time.Time) (consentBinding, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return consentBinding{}, errors.New("malformed")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return consentBinding{}, errors.New("malformed payload")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return consentBinding{}, errors.New("malformed signature")
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(payloadBytes)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return consentBinding{}, errors.New("bad signature")
	}
	fields := strings.Split(string(payloadBytes), consentSep)
	if len(fields) != 5 {
		return consentBinding{}, errors.New("bad payload")
	}
	issuedAt, err := strconv.ParseInt(fields[4], 10, 64)
	if err != nil {
		return consentBinding{}, errors.New("bad issued_at")
	}
	if now.Unix()-issuedAt > int64(consentTTL.Seconds()) {
		return consentBinding{}, errors.New("expired")
	}
	return consentBinding{
		ClientID: fields[0], RedirectURI: fields[1], Challenge: fields[2],
		State: fields[3], IssuedAt: issuedAt,
	}, nil
}
