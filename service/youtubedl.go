package service

import (
	"errors"
	"log"
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
