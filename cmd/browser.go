package cmd

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
)

// openBrowser opens rawURL in the user's default browser. Only http(s)
// URLs are accepted so a hostile API response can never smuggle a
// different URL scheme to the OS opener.
func openBrowser(rawURL string) error {
	parsed, parseError := url.Parse(rawURL)
	if parseError != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("refusing to open non-HTTP URL: %s", rawURL)
	}

	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", rawURL).Start()
	case "linux":
		return exec.Command("xdg-open", rawURL).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL).Start()
	default:
		return fmt.Errorf("unsupported platform %s", runtime.GOOS)
	}
}
