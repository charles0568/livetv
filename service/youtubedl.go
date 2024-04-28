package service

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/zjyl1994/livetv/model"

	"github.com/zjyl1994/livetv/global"
	"github.com/zjyl1994/livetv/plugin"
)

var errNoMatchFound error = errors.New("This channel is not currently live")

const DefaultUserAgent string = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"

func GetLiveM3U8(youtubeURL string, Parser string) (string, string, error) {
	liveInfo, ok := global.URLCache.Load(youtubeURL)
	if ok {
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

// returns: content, updated m3u8url (if needed), error
func GetM3U8Content(ChannelURL string, liveM3U8 string, Parser string) (string, string, error) {
	retry := func(bodyString string, err error) (string, string, error) {
		newUrl := liveM3U8
		if GetStatus(ChannelURL).Status == Ok {
			// this channel was previously running ok, we give it a chance to reparse itself
			log.Println(ChannelURL, "is unhealthy, doing a reparse...")
			if li, err := UpdateURLCacheSingle(ChannelURL, Parser); err == nil {
				UpdateStatus(ChannelURL, Warning, "Unhealthy")
				bodyString, newUrl, err = GetM3U8Content(ChannelURL, li.LiveUrl, Parser)
				log.Println("new url!", newUrl)
				if err == nil {
					log.Println(ChannelURL, "is back online now")
					UpdateStatus(ChannelURL, Ok, "Live!") // revert our temporary warning status to ok
				} else {
					log.Println(ChannelURL, "is still unhealthy, giving up, currently pointing to", liveM3U8)
				}
				// if error still persists after a reparse, keep our warning status so that we won't endlessly reparse the same feed
			}
		}
		return bodyString, newUrl, err
	}

	li, _ := global.URLCache.Load(ChannelURL)

	// allow plugins to decorate the m3u8 url
	decoraUrl := liveM3U8
	if p, err := plugin.GetPlugin(Parser); err == nil {
		if transformer, ok := p.(plugin.Transformer); ok {
			if li != nil {
				decoraUrl, _ = transformer.Transform(liveM3U8, li.ExtraInfo)
				log.Println("transformed", liveM3U8, "=>", decoraUrl)
			}
		}
	}

	client := http.Client{Timeout: global.HttpClientTimeout}
	req, err := http.NewRequest(http.MethodGet, decoraUrl, nil)
	if err != nil {
		log.Println(err)
		return "", liveM3U8, err
	}
	req.Header.Set("User-Agent", DefaultUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return "", liveM3U8, err
	}

	bodyString := ""
	defer resp.Body.Close()
	if resp.ContentLength < 10*1024*1024 && strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "mpegurl") {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", liveM3U8, err
		}
		bodyString = string(bodyBytes)
		// retry on server status error
		if resp.StatusCode != http.StatusOK {
			return retry(bodyString, errors.New(fmt.Sprintf("Server response: HTTP %d", resp.StatusCode)))
		}
		// retry on custom health check error
		if p, err := plugin.GetPlugin(Parser); err == nil {
			if checker, ok := p.(plugin.HealthCheck); ok {
				healthErr := checker.Check(bodyString, li)
				if healthErr != nil {
					return retry(bodyString, healthErr)
				}
			}
		}
	} else {
		UpdateStatus(ChannelURL, Warning, "Url is not a live stream")
		duration, err := GetVideoDuration(ChannelURL)
		if err == nil && duration > 0 {
			log.Println(ChannelURL, "duration is", duration)
			bodyString = fmt.Sprintf("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:%.0f\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXT-X-MEDIA-SEQUENCE:0\n#EXTINF:%.4f, video\n%s\n#EXT-X-ENDLIST", duration, duration, liveM3U8)
		} else {
			log.Println("failed to get duration", err.Error())
			bodyString = "#EXTM3U\n#EXTINF:-1, video\n#EXT-X-PLAYLIST-TYPE:VOD\n" + liveM3U8 + "\n#EXT-X-ENDLIST" // make a fake m3u8 pointing to the target
		}
	}
	return bodyString, liveM3U8, nil
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
