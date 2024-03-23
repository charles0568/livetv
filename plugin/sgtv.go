// sgtv
package plugin

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dlclark/regexp2"
)

type FTVParser struct{}

type FTVResponse struct {
	Ad       string
	VideoURL string
	fsVENDOR string
}

func (p *FTVParser) Parse(liveUrl string) (string, string, error) {
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
	// DO not parse invalid response, parse HTML only
	if resp.ContentLength > 10*1024*1024 || !strings.Contains(resp.Header.Get("Content-Type"), "html") {
		return "", "", errors.New("invalid response")
	}
	content, _ := io.ReadAll(resp.Body)
	scontent := string(content)
	regex := regexp2.MustCompile(`\((.+)\)$`, 0)
	matches, _ := regex.FindStringMatch(scontent)
	if matches != nil {
		gps := matches.Groups()
		resultJson := gps[1].Captures[0].String()
		var ftvresp FTVResponse
		json.Unmarshal([]byte(resultJson), &ftvresp)
		if ftvresp.VideoURL != "" {
			liveUrl, err := bestFromMasterPlaylist(ftvresp.VideoURL)
			if err == nil {
				if !isValidURL(liveUrl) {
					liveUrl = getBaseURL(liveUrl) + liveUrl
				}
				return liveUrl, "", nil
			}
			return "", "", err
		}
	}
	return "", "", NoMatchFeed
}

func init() {
	registerPlugin("FTV", &FTVParser{})
}
