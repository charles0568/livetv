// utils
package global

import (
	"net/url"
	"path"
)

func GetBaseURL(rawURL string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	parsedURL.RawQuery = ""

	// Remove the last element (document) from the path
	parsedURL.Path = path.Dir(parsedURL.Path) + "/"

	// Rebuild the URL without the document part
	return parsedURL.String()
}

func IsValidURL(u string) bool {
	_, err := url.ParseRequestURI(u)
	return err == nil
}
