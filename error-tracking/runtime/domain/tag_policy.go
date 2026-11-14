package observabilitydomain

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Agent-facing event tags are a bounded, allowlisted projection. Sentry SDKs
// attach arbitrary application tags and some of them carry personal data, so
// agent reads and tag searches never expose a tag merely because it was stored.
const (
	MaxPublicTags           = 50
	MaxPublicTagValueLength = 200
)

var publicTagKeyPattern = regexp.MustCompile(`^[a-z0-9_.]{1,64}$`)

// deniedTagKeys are exact keys that identify a person, a client address or a
// full URL (which can carry query-string personal data or capability tokens).
var deniedTagKeys = map[string]struct{}{
	"email": {}, "user.email": {}, "user_email": {}, "e_mail": {},
	"ip": {}, "ip_address": {}, "user.ip": {}, "user.ip_address": {}, "user_ip": {},
	"client_ip": {}, "client.ip": {}, "remote_addr": {}, "remote_ip": {},
	"x_forwarded_for": {}, "forwarded_for": {}, "http.client_ip": {},
	"username": {}, "user.username": {}, "user_name": {}, "user.name": {},
	"name": {}, "full_name": {}, "first_name": {}, "last_name": {},
	"user": {}, "user.id.email": {},
	"url": {}, "request.url": {}, "http.url": {}, "full_url": {}, "uri": {},
	"request.uri": {}, "http.request.url": {}, "referer": {}, "referrer": {},
	"http.referer": {}, "request.referer": {}, "query": {}, "query_string": {},
	"request.query_string": {}, "http.query": {},
	"phone": {}, "user.phone": {}, "phone_number": {}, "mobile": {},
	"address": {}, "street": {}, "postcode": {}, "postal_code": {}, "zip": {},
	"zipcode": {}, "geo": {}, "user.geo": {}, "location": {},
	"device.id": {}, "device_id": {}, "user_agent": {}, "http.user_agent": {},
	"browser.user_agent": {},
}

// deniedTagKeyParts reject any key containing these fragments: credentials,
// session material and contact details have no safe agent-facing value.
var deniedTagKeyParts = []string{
	"email", "password", "passwd", "secret", "token", "cookie", "authorization",
	"api_key", "apikey", "credential", "session",
}

// deniedTagKeySegments reject a key when any "."/"_"-separated segment matches,
// so "origin.ip", "client_ip", "callback_url" and "user.phone" stay private
// without rejecting unrelated words that merely contain the same letters.
var deniedTagKeySegments = map[string]struct{}{
	"ip": {}, "ipv4": {}, "ipv6": {}, "url": {}, "uri": {}, "referer": {}, "referrer": {},
	"phone": {}, "mobile": {}, "address": {}, "geo": {}, "ssn": {}, "bsn": {},
	"iban": {}, "passport": {}, "username": {},
}

var emailLikeValue = regexp.MustCompile(`[^\s@]+@[^\s@]+\.[^\s@]+`)

// IsPublicTagKey reports whether a tag key may be returned to, or searched by,
// an agent. It applies the conservative key pattern and the personal-data deny
// rules; it does not inspect values.
func IsPublicTagKey(key string) bool {
	if !publicTagKeyPattern.MatchString(key) {
		return false
	}
	if _, denied := deniedTagKeys[key]; denied {
		return false
	}
	for _, part := range deniedTagKeyParts {
		if strings.Contains(key, part) {
			return false
		}
	}
	for _, segment := range strings.FieldsFunc(key, func(r rune) bool { return r == '.' || r == '_' }) {
		if _, denied := deniedTagKeySegments[segment]; denied {
			return false
		}
	}
	return true
}

// IsPublicTagValue reports whether a stored value is bounded, printable and not
// an email-shaped string. Over-long values are omitted, not truncated, so an
// exact value used for correlation is never silently altered.
func IsPublicTagValue(value string) bool {
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > MaxPublicTagValueLength {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return !emailLikeValue.MatchString(value)
}

// PublicEventTags returns at most MaxPublicTags allowlisted tags, choosing by
// key order so the projection is deterministic. truncated reports that more
// allowlisted tags existed than were returned.
func PublicEventTags(tags map[string]string) (public map[string]string, truncated bool) {
	public = map[string]string{}
	keys := make([]string, 0, len(tags))
	for key, value := range tags {
		if IsPublicTagKey(key) && IsPublicTagValue(value) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if len(keys) > MaxPublicTags {
		keys, truncated = keys[:MaxPublicTags], true
	}
	for _, key := range keys {
		public[key] = tags[key]
	}
	return public, truncated
}
