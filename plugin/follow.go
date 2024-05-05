// follow location
package plugin

import (
	"net/http"
	"time"

	"github.com/zjyl1994/livetv/model"
)

type URLM3U8Parser struct{}

func (p *URLM3U8Parser) Parse(liveUrl string, proxyUrl string, lastInfo string) (*model.LiveInfo, error) {
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
	redir := resp.Header.Get("Location")
	if redir == "" {
		return nil, NoMatchFeed
	}
	directParser := &DirectM3U8Parser{}
	return directParser.Parse(redir, proxyUrl, lastInfo)
}

func init() {
	registerPlugin("httpRedirect", &URLM3U8Parser{})
}
