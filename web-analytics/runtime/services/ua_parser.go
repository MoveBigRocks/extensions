package analyticsservices

import (
	"github.com/mssola/useragent"
)

// ParseUA extracts browser and OS names with versions, plus device type.
func ParseUA(uaString string) (browser, browserVersion, os, osVersion, deviceType string) {
	if uaString == "" {
		return "", "", "", "", ""
	}

	ua := useragent.New(uaString)

	browserName, parsedBrowserVersion := ua.Browser()
	browser = browserName
	browserVersion = parsedBrowserVersion

	os = ua.OS()
	osInfo := ua.OSInfo()
	osVersion = osInfo.Version

	if ua.Mobile() {
		deviceType = "Mobile"
	} else if ua.Bot() {
		deviceType = "Bot"
	} else {
		// useragent library doesn't distinguish tablet; default to Desktop
		deviceType = "Desktop"
	}

	// Detect tablets from OS hints
	if deviceType == "Desktop" {
		if os == "iPad" || os == "iPadOS" {
			deviceType = "Tablet"
		}
	}

	return browser, browserVersion, os, osVersion, deviceType
}
