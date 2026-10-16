package api

import "strings"

// describeDevice tells from a User-Agent what kind of device a request
// came from ("phone", "tablet", "desktop", or "" if unknown) and reads it
// out like "Firefox on Windows".
func describeDevice(ua string) (kind, name string) {
	has := func(s string) bool { return strings.Contains(ua, s) }
	os := ""
	switch {
	case has("iPad"):
		os, kind = "iPadOS", "tablet"
	case has("iPhone"):
		os, kind = "iOS", "phone"
	case has("Android"):
		os, kind = "Android", "tablet"
		// Android phones send "Mobile"; tablets don't.
		if has("Mobile") {
			kind = "phone"
		}
	case has("Windows"):
		os, kind = "Windows", "desktop"
	case has("CrOS"):
		os, kind = "ChromeOS", "desktop"
	case has("Macintosh"):
		os, kind = "macOS", "desktop"
	case has("Linux"):
		os, kind = "Linux", "desktop"
	}
	browser := ""
	switch {
	case has("Edg/"), has("EdgA/"), has("EdgiOS/"):
		browser = "Edge"
	case has("OPR/"):
		browser = "Opera"
	case has("SamsungBrowser/"):
		browser = "Samsung Internet"
	case has("Firefox/"), has("FxiOS/"):
		browser = "Firefox"
	case has("Chrome/"), has("CriOS/"):
		browser = "Chrome"
	case has("Safari/"):
		browser = "Safari"
	case strings.HasPrefix(ua, "curl/"):
		browser = "curl"
	}
	switch {
	case browser != "" && os != "":
		return kind, browser + " on " + os
	case browser != "":
		return kind, browser
	case os != "":
		return kind, os
	}
	return "", ""
}
