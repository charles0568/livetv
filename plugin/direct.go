// direct
package plugin

import (
	"log"
	"net/http"
	"strings"
	"time"
)

type DirectM3U8Parser struct{}

func (p *DirectM3U8Parser) Parse(liveUrl string) (string, string, error) {
	client := http.Client{
		Timeout: time.Second * 10,
	}
	req, err := http.NewRequest("GET", liveUrl, nil)
	req.Header.Set("User-Agent", DefaultUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}

	defer resp.Body.Close()
	// the link itself is a valid M3U8
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "mpegurl") {
		log.Println(liveUrl, "is valid url")
		liveUrl, err := bestFromMasterPlaylist(liveUrl, resp.Body) // extract the best quality live url from the master playlist
		if err == nil {
			if !isValidURL(liveUrl) {
				liveUrl = getBaseURL(liveUrl) + liveUrl
			}
			return liveUrl, "", nil
		}
	}
	return "", "", NoMatchFeed
}

func init() {
	registerPlugin("direct", &DirectM3U8Parser{})
}
