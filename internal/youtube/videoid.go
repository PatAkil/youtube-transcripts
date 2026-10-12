package youtube

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// videoIDPattern matches a complete YouTube video ID: eleven characters from
// the URL-safe base64 alphabet.
var videoIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// ParseVideoID extracts a YouTube video ID from a bare ID or from any of the
// usual YouTube URL forms: watch?v=, youtu.be/<id>, /shorts/<id> and
// /embed/<id>, with or without a scheme and with any extra query parameters.
func ParseVideoID(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("empty video reference")
	}
	if videoIDPattern.MatchString(s) {
		return s, nil
	}

	raw := s
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("not a YouTube video ID or URL: %q", s)
	}

	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	host = strings.TrimPrefix(host, "m.")
	segments := strings.Split(strings.Trim(u.Path, "/"), "/")

	var candidate string
	switch host {
	case "youtu.be":
		candidate = segments[0]
	case "youtube.com", "youtube-nocookie.com":
		switch {
		case segments[0] == "watch":
			candidate = u.Query().Get("v")
		case len(segments) >= 2 && (segments[0] == "shorts" || segments[0] == "embed" || segments[0] == "live" || segments[0] == "v"):
			candidate = segments[1]
		}
	default:
		return "", fmt.Errorf("not a YouTube video ID or URL: %q", s)
	}

	if !videoIDPattern.MatchString(candidate) {
		return "", fmt.Errorf("no video ID found in %q", s)
	}
	return candidate, nil
}
