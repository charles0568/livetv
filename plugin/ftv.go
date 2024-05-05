// ftv
package plugin

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/zjyl1994/livetv/model"

	"github.com/dlclark/regexp2"
)

type FTVParser struct{}

type FTVResponse struct {
	Ad       string
	VideoURL string
	fsVENDOR string
}

func (p *FTVParser) Parse(liveUrl string, proxyUrl string, lastInfo string) (*model.LiveInfo, error) {
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
	// DO not parse invalid response, parse HTML only
	if resp.ContentLength > 10*1024*1024 || !strings.Contains(resp.Header.Get("Content-Type"), "html") {
		return nil, errors.New("invalid response")
	}
	content, _ := io.ReadAll(resp.Body)
	scontent := string(content)
	regex := regexp2.MustCompile(`\((.+)\)$`, 0)
	matches, _ := regex.FindStringMatch(scontent)
	if matches != nil {
		gps := matches.Groups()
		resultJson := gps[1].Captures[0].String()
		log.Println("response json:", resultJson)
		var ftvresp FTVResponse
		json.Unmarshal([]byte(resultJson), &ftvresp)
		if ftvresp.VideoURL != "" {
			liveUrl, err := bestFromMasterPlaylist(ftvresp.VideoURL)
			log.Println("best url:", liveUrl)
			li := &model.LiveInfo{}
			if err == nil {
				li.LiveUrl = liveUrl
				return li, nil
			}
			return nil, err
		}
	}
	return nil, NoMatchFeed
}

func init() {
	registerPlugin("FTV", &FTVParser{})
}
