package analyticsdomain

import (
	"crypto/rand"
	"encoding/binary"
	"net/url"
	"strings"
	"time"
)

var knownSources = map[string]string{
	"google.com":           "Google",
	"google.co.uk":         "Google",
	"google.nl":            "Google",
	"google.com.au":        "Google",
	"www.google.com":       "Google",
	"www.google.co.uk":     "Google",
	"www.google.nl":        "Google",
	"www.google.com.au":    "Google",
	"bing.com":             "Bing",
	"www.bing.com":         "Bing",
	"duckduckgo.com":       "DuckDuckGo",
	"ecosia.org":           "Ecosia",
	"www.ecosia.org":       "Ecosia",
	"linkedin.com":         "LinkedIn",
	"www.linkedin.com":     "LinkedIn",
	"lnkd.in":              "LinkedIn",
	"twitter.com":          "Twitter",
	"www.twitter.com":      "Twitter",
	"x.com":                "Twitter",
	"t.co":                 "Twitter",
	"facebook.com":         "Facebook",
	"www.facebook.com":     "Facebook",
	"l.facebook.com":       "Facebook",
	"reddit.com":           "Reddit",
	"www.reddit.com":       "Reddit",
	"old.reddit.com":       "Reddit",
	"github.com":           "GitHub",
	"youtube.com":          "YouTube",
	"www.youtube.com":      "YouTube",
	"news.ycombinator.com": "Hacker News",
	"chatgpt.com":          "ChatGPT",
	"perplexity.ai":        "Perplexity",
}

type SessionParams struct {
	PropertyID     string
	VisitorID      int64
	Hostname       string
	Pathname       string
	Referrer       string
	ReferrerHost   string
	ReferrerSource string
	Channel        string
	UTMSource      string
	UTMMedium      string
	UTMCampaign    string
	UTMTerm        string
	UTMContent     string
	CountryCode    string
	Region         string
	City           string
	Browser        string
	BrowserVersion string
	OS             string
	OSVersion      string
	DeviceType     string
	StartedAt      time.Time
	EventName      string
}

// ClassifySource determines the traffic source from UTM parameters and referrer.
func ClassifySource(utmSource, referrer, propertyDomain string) string {
	if utmSource != "" {
		return utmSource
	}

	if referrer == "" {
		return "Direct"
	}

	parsed, err := url.Parse(referrer)
	if err != nil || parsed.Host == "" {
		return "Direct"
	}

	host := strings.ToLower(parsed.Hostname())
	propertyDomain = strings.ToLower(propertyDomain)

	if host == propertyDomain || strings.HasSuffix(host, "."+propertyDomain) {
		return ""
	}

	if name, ok := knownSources[host]; ok {
		return name
	}

	return strings.TrimPrefix(host, "www.")
}

// ClassifyChannel groups traffic into broad acquisition channels.
func ClassifyChannel(utmSource, utmMedium, referrer, propertyDomain string) string {
	medium := strings.ToLower(strings.TrimSpace(utmMedium))
	switch medium {
	case "cpc", "ppc", "paidsearch", "paid-search", "sem":
		return "Paid Search"
	case "paid-social", "paid_social", "paidsocial":
		return "Paid Social"
	case "social", "social-media", "social_media", "social-network", "social_network":
		return "Social"
	case "email", "e-mail":
		return "Email"
	case "display", "banner", "cpm":
		return "Display"
	case "affiliate":
		return "Affiliate"
	case "sms", "text", "text-message", "message":
		return "SMS"
	case "referral":
		return "Referral"
	}

	if referrer == "" {
		return "Direct"
	}

	source := ClassifySource(utmSource, referrer, propertyDomain)
	switch source {
	case "":
		return ""
	case "Direct":
		return "Direct"
	case "Google", "Bing", "DuckDuckGo", "Ecosia":
		return "Organic Search"
	case "Twitter", "Facebook", "LinkedIn", "Reddit", "YouTube":
		return "Social"
	case "ChatGPT", "Perplexity":
		return "AI"
	default:
		return "Referral"
	}
}

// CountryFromLanguage extracts a 2-letter country code from Accept-Language.
func CountryFromLanguage(acceptLang string) string {
	if acceptLang == "" {
		return ""
	}

	parts := strings.SplitN(acceptLang, ",", 2)
	tag := strings.TrimSpace(parts[0])
	tag = strings.SplitN(tag, ";", 2)[0]

	if idx := strings.IndexByte(tag, '-'); idx >= 0 {
		region := tag[idx+1:]
		if len(region) == 2 {
			return strings.ToUpper(region)
		}
	}
	return ""
}

// ParseTrackedURL extracts the tracked hostname, pathname, and UTM parameters.
func ParseTrackedURL(rawURL string) (hostname, pathname, utmSource, utmMedium, utmCampaign, utmTerm, utmContent string) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", "/", "", "", "", "", ""
	}

	hostname = strings.ToLower(parsed.Hostname())
	pathname = parsed.Path
	if pathname == "" {
		pathname = "/"
	}

	query := parsed.Query()
	return hostname, pathname, query.Get("utm_source"), query.Get("utm_medium"), query.Get("utm_campaign"), query.Get("utm_term"), query.Get("utm_content")
}

// ExtractTrackedHostname extracts the normalized hostname from a tracked URL.
func ExtractTrackedHostname(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}

// RecordActivity updates session metrics for a newly ingested event.
func (s *Session) RecordActivity(eventName, pathname string, at time.Time) {
	s.LastActivity = at
	s.Duration = int(at.Sub(s.StartedAt).Seconds())
	s.EventCount++

	if eventName == "pageview" {
		s.ExitPage = pathname
		s.Pageviews++
		if s.Pageviews > 1 {
			s.IsBounce = 0
		}
	}
}

// NewSessionFromIngest builds the initial session aggregate for an ingested event.
func NewSessionFromIngest(params SessionParams) *Session {
	pageviews := 0
	if params.EventName == "pageview" {
		pageviews = 1
	}

	return &Session{
		SessionID:      sessionID(),
		PropertyID:     params.PropertyID,
		VisitorID:      params.VisitorID,
		Hostname:       params.Hostname,
		EntryPage:      params.Pathname,
		ExitPage:       params.Pathname,
		Referrer:       params.Referrer,
		ReferrerHost:   params.ReferrerHost,
		ReferrerSource: params.ReferrerSource,
		Channel:        params.Channel,
		UTMSource:      params.UTMSource,
		UTMMedium:      params.UTMMedium,
		UTMCampaign:    params.UTMCampaign,
		UTMTerm:        params.UTMTerm,
		UTMContent:     params.UTMContent,
		CountryCode:    params.CountryCode,
		Region:         params.Region,
		City:           params.City,
		Browser:        params.Browser,
		BrowserVersion: params.BrowserVersion,
		OS:             params.OS,
		OSVersion:      params.OSVersion,
		DeviceType:     params.DeviceType,
		StartedAt:      params.StartedAt,
		LastActivity:   params.StartedAt,
		Duration:       0,
		Pageviews:      pageviews,
		EventCount:     1,
		IsBounce:       1,
	}
}

func sessionID() int64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0
	}
	return int64(binary.BigEndian.Uint64(b[:]))
}
