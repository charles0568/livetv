package service

import (
	"log"
	"regexp"
	"time"

	"github.com/zjyl1994/livetv/global"
	"github.com/zjyl1994/livetv/util"
)

func LoadChannelCache() {
	channels, err := GetAllChannel()
	if err != nil {
		log.Println(err)
		return
	}
	for _, v := range channels {
		UpdateURLCacheSingle(v.URL)
	}
}

func UpdateURLCacheSingle(Url string) (string, error) {
	log.Println("caching", Url)
	liveURL, err := RealGetYoutubeLiveM3U8(Url)
	if err != nil {
		log.Println("[YTDL]", err)
		UpdateStatus(Url, Error, err.Error())
	} else {
		global.URLCache.Store(Url, liveURL)
		UpdateStatus(Url, Ok, "Live!")
		log.Println(Url, "cached")
	}
	return liveURL, err
}

func UpdateURLCache() {
	channels, err := GetAllChannel()
	if err != nil {
		log.Println(err)
		return
	}
	global.URLCache.Range(func(k, v interface{}) bool {
		value := v.(string)
		regex := regexp.MustCompile(`/expire/(\d+)/`)
		matched := regex.FindStringSubmatch(value)
		if len(matched) < 2 {
			global.URLCache.Delete(k)
			DeleteStatus(k)
			return true
		}
		expireTime := time.Unix(util.String2Int64(matched[1]), 0)
		if time.Now().After(expireTime) {
			global.URLCache.Delete(k)
			DeleteStatus(k)
		}
		return true
	})
	for _, v := range channels {
		UpdateURLCacheSingle(v.URL)
	}
}
