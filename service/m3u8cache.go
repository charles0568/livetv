package service

import (
	"log"
	"regexp"
	"time"

	"github.com/zjyl1994/livetv/model"

	"github.com/zjyl1994/livetv/global"
	"github.com/zjyl1994/livetv/util"
)

var updateConcurrent = make(chan bool, 2) // allow up to 2 urls to be updated simultaneously

func LoadChannelCache() {
	UpdateChannelDB() // update database structure
	channels, err := GetAllChannel()
	if err != nil {
		log.Println(err)
		return
	}
	for _, v := range channels {
		UpdateURLCacheSingle(v.URL, v.Parser)
	}
}

func UpdateURLCacheSingle(Url string, Parser string) (*model.LiveInfo, error) {
	updateConcurrent <- true
	defer func() {
		<-updateConcurrent
	}()
	log.Println("caching", Url)
	liveInfo, err := RealLiveM3U8(Url, Parser)
	if err != nil {
		global.URLCache.Delete(Url)
		UpdateStatus(Url, Error, err.Error())
		log.Println("[LiveTV]", err)
	} else {
		global.URLCache.Store(Url, liveInfo)
		UpdateStatus(Url, Ok, "Live!")
		log.Println(Url, "cached")
	}
	return liveInfo, err
}

func UpdateURLCache() {
	channels, err := GetAllChannel()
	if err != nil {
		log.Println(err)
		return
	}
	global.URLCache.Range(func(k string, info *model.LiveInfo) bool {
		regex := regexp.MustCompile(`/expire/(\d+)/`)
		matched := regex.FindStringSubmatch(info.LiveUrl)
		if len(matched) < 2 {
			global.URLCache.Delete(k)
			DeleteStatus(k)
			return true
		}
		expireTime := time.Unix(util.String2Int64(matched[1]), 0)
		if time.Now().Add(time.Hour * 4).After(expireTime) {
			global.URLCache.Delete(k)
			DeleteStatus(k)
		}
		return true
	})
	for _, v := range channels {
		UpdateURLCacheSingle(v.URL, v.Parser)
	}
}
