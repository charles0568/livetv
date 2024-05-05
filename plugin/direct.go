// direct
package plugin

import (
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/zjyl1994/livetv/global"
	"github.com/zjyl1994/livetv/model"
)

type DirectM3U8Parser struct{}

func (p *DirectM3U8Parser) Parse(liveUrl string, proxyUrl string, lastInfo string) (*model.LiveInfo, error) {
	u, err := url.Parse(liveUrl)
	// return non http protocol directly
	if err == nil && !strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Scheme, "https") {
		li := &model.LiveInfo{}
		li.LiveUrl = liveUrl
		return li, nil
	}

	client := http.Client{
		Timeout:   time.Second * 10,
		Transport: transportWithProxy(proxyUrl),
	}
	req, err := http.NewRequest("GET", liveUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", DefaultUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	// the link itself is a valid M3U8
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "mpegurl") {
		log.Println(liveUrl, "is a valid url")
		liveUrl, err := bestFromMasterPlaylist(liveUrl, resp.Body) // extract the best quality live url from the master playlist
		if err == nil {
			li := &model.LiveInfo{}
			if !global.IsValidURL(liveUrl) {
				liveUrl = global.GetBaseURL(liveUrl) + liveUrl
			}
			li.LiveUrl = liveUrl
			return li, nil
		}
	}
	return nil, NoMatchFeed
}

func init() {
	registerPlugin("direct", &DirectM3U8Parser{})
}
