// youtube
package plugin

import (
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dlclark/regexp2"
)

type YoutubeParser struct{}

func (p *YoutubeParser) Parse(liveUrl string) (string, string, error) {
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
		log.Println(liveUrl, "is a master url")
		liveUrl, err := bestFromMasterPlaylist(liveUrl, resp.Body) // extract the best quality live url from the master playlist
		if err == nil {
			if !isValidURL(liveUrl) {
				liveUrl = getBaseURL(liveUrl) + liveUrl
			}
			return liveUrl, "", nil
		}
	}
	// DO not parse invalid response, parse HTML only
	if resp.ContentLength > 10*1024*1024 || !strings.Contains(resp.Header.Get("Content-Type"), "html") {
		return "", "", errors.New("invalid url")
	}
	content, _ := io.ReadAll(resp.Body)
	scontent := string(content)
	regex := regexp2.MustCompile(`(?<=hlsManifestUrl":").*\.m3u8`, 0)
	matches, _ := regex.FindStringMatch(scontent)
	if matches != nil {
		gps := matches.Groups()
		liveMasterUrl := gps[0].Captures[0].String()
		liveUrl, err := bestFromMasterPlaylist(liveMasterUrl) // extract the best quality live url from the master playlist
		if err != nil {
			return "", "", err
		}

		logo := ""
		logoexp := regexp2.MustCompile(`(?<=owner":{"videoOwnerRenderer":{"thumbnail":{"thumbnails":\[{"url":")[^=]*`, 0)
		logomatches, _ := logoexp.FindStringMatch(scontent)
		if logomatches != nil {
			logo = logomatches.Groups()[0].Captures[0].String()
		}
		return liveUrl, logo, nil
	}
	return "", "", NoMatchFeed
}

func init() {
	registerPlugin("youtube", &YoutubeParser{})
}
