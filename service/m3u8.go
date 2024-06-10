package service

import (
	"bufio"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/zjyl1994/livetv/global"
	"github.com/zjyl1994/livetv/util"
)

var startUp int64 = 0

func PlaceHolderHLS() string {
	// t := (time.Now().Unix() - startUp) / 60
	baseUrl, _ := global.GetConfig("base_url")
	if !strings.HasSuffix(baseUrl, "/") {
		baseUrl += "/"
	}
	placeholder := baseUrl + "placeholder.ts"
	tpl := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-MEDIA-SEQUENCE:1
#EXT-X-TARGETDURATION:30
#EXT-X-DISCONTINUITY:0
#EXTINF:30.000000,
%s?t=1
#EXTINF:30.000000,
%s?t=2
#EXTINF:30.000000,
%s?t=3
#EXT-X-ENDLIST
`
	return fmt.Sprintf(tpl, placeholder, placeholder, placeholder)
}

func cleanUrl(Url string) string {
	parsedURL, err := url.Parse(Url)
	if err != nil {
		return Url
	}

	// Resolve the path using path resolution
	parsedURL.Path = path.Clean(parsedURL.Path) // Remove trailing segments

	// Get the final clean URL as a string
	cleanURL := parsedURL.String()

	return cleanURL
}

func M3U8Process(playlistUrl string, data string, prefixURL string, proxy bool, fnTransform func(raw string, ts string) string) string {
	var sb strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(data))
	baseUrl := global.GetBaseURL(playlistUrl)
	for scanner.Scan() {
		l := strings.TrimSpace(scanner.Text())
		if l == "" {
			continue
		}
		if strings.HasPrefix(l, "#") {
			sb.WriteString(l)
		} else {
			if !global.IsValidURL(l) {
				l = cleanUrl(baseUrl + l)
			}
			if proxy {
				tsLink := prefixURL + util.CompressString(l)
				if fnTransform != nil {
					tsLink = fnTransform(l, tsLink)
				}
				sb.WriteString(tsLink)
			} else {
				sb.WriteString(l)
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func init() {
	startUp = time.Now().Unix()
}
