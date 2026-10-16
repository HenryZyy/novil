package api

import "testing"

func TestDescribeDevice(t *testing.T) {
	for ua, want := range map[string][2]string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:131.0) Gecko/20100101 Firefox/131.0":                                                        {"desktop", "Firefox on Windows"},
		"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Mobile Safari/537.36":                   {"phone", "Chrome on Android"},
		"Mozilla/5.0 (Linux; Android 13; SM-X700) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36":                          {"tablet", "Chrome on Android"},
		"Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1": {"phone", "Safari on iOS"},
		"Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1":          {"tablet", "Safari on iPadOS"},
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36 Edg/130.0.0.0":     {"desktop", "Edge on macOS"},
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36":                                   {"desktop", "Chrome on Linux"},
		"curl/8.5.0": {"", "curl"},
		"":           {"", ""},
	} {
		kind, name := describeDevice(ua)
		if kind != want[0] || name != want[1] {
			t.Errorf("%q: %q %q, want %q %q", ua, kind, name, want[0], want[1])
		}
	}
}
