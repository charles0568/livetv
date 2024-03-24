package service

import (
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/zjyl1994/livetv/model"

	"github.com/grafov/m3u8"
	"github.com/zjyl1994/livetv/global"
	"github.com/zjyl1994/livetv/plugin"
)

var errNoMatchFound error = errors.New("This channel is not currently live")

const DefaultUserAgent string = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"

/*
func checkAndUpdateExpiringM3U8(youtubeURL string, liveURL string) (expired bool) {
	regex := regexp.MustCompile(`/expire/(\d+)/`)
	matched := regex.FindStringSubmatch(liveURL)
	if len(matched) < 2 {
		return false
	}
	expireTime := time.Unix(util.String2Int64(matched[1]), 0)
	if time.Now().After(expireTime) { // already expired, update before replying to clients
		global.URLCache.Delete(youtubeURL)
		UpdateURLCacheSingle(youtubeURL)
		return true
	} else if time.Now().Add(time.Minute * 6).After(expireTime) {
		go UpdateURLCacheSingle(youtubeURL) // update async
	}
	return false
}*/

func GetLiveM3U8(youtubeURL string, Parser string) (string, string, error) {
	liveInfo, ok := global.URLCache.Load(youtubeURL)
	if ok {
		// check and refresh expired/expiring feed
		/*if checkAndUpdateExpiringM3U8(youtubeURL, liveURL) {
			// expired link, should load liveUrl again
			liveURL, ok = global.URLCache.Load(youtubeURL)
			if !ok {
				return "", "", errNoMatchFound
			}
		}*/
		return liveInfo.LiveUrl, liveInfo.Logo, nil
	} else {
		log.Println("cache miss", youtubeURL)
		status := GetStatus(youtubeURL)
		if time.Now().Sub(status.Time) > time.Minute*2 {
			if liveInfo, err := UpdateURLCacheSingle(youtubeURL, Parser); err == nil {
				return liveInfo.LiveUrl, liveInfo.Logo, nil
			} else {
				return "", "", err
			}
		} else {
			return "", "", errors.New("parser cooling down")
		}
	}
}

func getBaseURL(rawURL string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	// Remove the last element (document) from the path
	parsedURL.Path = path.Dir(parsedURL.Path) + "/"

	// Rebuild the URL without the document part
	return parsedURL.String()
}

func isValidURL(u string) bool {
	_, err := url.ParseRequestURI(u)
	return err == nil
}

func bestFromMasterPlaylist(masterUrl string, content ...io.Reader) (string, error) {
	var playlist io.Reader
	if len(content) > 0 {
		playlist = content[0]
	} else {
		client := http.Client{
			Timeout: time.Second * 10,
		}
		req, err := http.NewRequest("GET", masterUrl, nil)
		req.Header.Set("User-Agent", DefaultUserAgent)
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.ContentLength > 10*1024*1024 || !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "mpegurl") {
			return "", errors.New("invalid url")
		}
		playlist = resp.Body
	}
	p, listType, err := m3u8.DecodeFrom(playlist, true)
	if err != nil {
		return "", err
	}
	switch listType {
	case m3u8.MEDIA:
		{
			return masterUrl, nil
		}
	case m3u8.MASTER:
		{
			masterpl := p.(*m3u8.MasterPlaylist)
			selectedUrl := ""
			selectedBw := uint32(0)
			for _, v := range masterpl.Variants {
				if v.Bandwidth >= selectedBw {
					selectedUrl = v.URI
					selectedBw = v.Bandwidth
				}
			}
			return selectedUrl, nil
		}
	}
	return "", errors.New("Unknown type of playlist")
}

func RealLiveM3U8(liveUrl string, Parser string) (*model.LiveInfo, error) {
	if Parser == "" {
		Parser = "youtube" // backward compatible with old database, use youtube parser by default
	}
	if p, err := plugin.GetPlugin(Parser); err == nil {
		if liveInfo, ok := global.URLCache.Load(liveUrl); ok {
			return p.Parse(liveUrl, liveInfo.ExtraInfo)
		}
		return p.Parse(liveUrl, "")
	} else {
		return nil, err
	}
}
