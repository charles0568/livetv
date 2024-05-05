// thepaper
// ftv
package plugin

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/zjyl1994/livetv/global"
	"github.com/zjyl1994/livetv/model"
)

type ThePaperParser struct{}

type ThePaperPlayInfo struct {
	Title    string `json:"title"`
	Url      string `json:"url"`
	CoverUrl string `json:"coverUrl"`
}

type ThePaperContent struct {
	NowPlay  ThePaperPlayInfo `json:"nowPlay"`
	NextPlay ThePaperPlayInfo `json:"nextPlay"`
	NextTime int64            `json:"nextTime"`
	ImgUrl   string           `json:"imgUrl"`
}

type ThePaperResponse struct {
	Code int32           `json:"code"`
	Desc string          `json:"desc"`
	Time int64           `json:"time"`
	Data ThePaperContent `json:"data"`
}

func (p *ThePaperParser) Parse(liveUrl string, proxyUrl string, lastInfo string) (*model.LiveInfo, error) {
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
	// DO not parse invalid response, parse JSON only
	if resp.ContentLength > 10*1024*1024 || !strings.Contains(resp.Header.Get("Content-Type"), "json") {
		return nil, errors.New("invalid response")
	}
	content, _ := io.ReadAll(resp.Body)
	var paperResp ThePaperResponse
	if json.Unmarshal(content, &paperResp) == nil && paperResp.Code == 200 {
		li := &model.LiveInfo{}
		li.LiveUrl = paperResp.Data.NowPlay.Url
		li.Logo = paperResp.Data.ImgUrl
		if global.IsValidURL(li.LiveUrl) {
			return li, nil
		}
	}
	return nil, NoMatchFeed
}

func init() {
	registerPlugin("ThePaper", &ThePaperParser{})
}
