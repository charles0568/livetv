// follow location
package plugin

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/zjyl1994/livetv/model"
)

type URLM3U8Parser struct{}

func (p *URLM3U8Parser) Transform(req *http.Request, info *model.LiveInfo) error {
	directParser := &DirectM3U8Parser{}
	return directParser.Transform(req, info)
}

func (p *URLM3U8Parser) Parse(liveUrl string, proxyUrl string, previousExtraInfo string) (*model.LiveInfo, error) {
	client := http.Client{
		Timeout:   time.Second * 10,
		Transport: transportWithProxy(proxyUrl),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
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
	var ui UrlInfo
	decoder := json.NewDecoder(resp.Body)
	if decoder.Decode(&ui) == nil && len(ui.Headers) > 0 {
		js, _ := json.Marshal(ui)
		previousExtraInfo = string(js) // write headers info to extraInfo
	}

	redir := resp.Header.Get("Location")
	if redir == "" {
		return nil, NoMatchFeed
	}
	directParser := &DirectM3U8Parser{}
	return directParser.Parse(redir, proxyUrl, previousExtraInfo)
}

func init() {
	registerPlugin("httpRedirect", &URLM3U8Parser{})
}
