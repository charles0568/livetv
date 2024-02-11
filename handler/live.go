package handler

import (
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"github.com/zjyl1994/livetv/global"
	"github.com/zjyl1994/livetv/service"
	"github.com/zjyl1994/livetv/util"
)

func M3UHandler(c *gin.Context) {
	content, err := service.M3UGenerate()
	if err != nil {
		log.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.Data(http.StatusOK, "application/vnd.apple.mpegurl", []byte(content))
}

func LiveHandler(c *gin.Context) {
	var m3u8Body string
	channelCacheKey := c.Query("c")
	iBody, found := global.M3U8Cache.Get(channelCacheKey)
	if found {
		m3u8Body = iBody.(string)
	} else {
		channelNumber := util.String2Uint(c.Query("c"))
		if channelNumber == 0 {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		channelInfo, err := service.GetChannel(channelNumber)
		if err != nil {
			if gorm.IsRecordNotFoundError(err) {
				c.AbortWithStatus(http.StatusNotFound)
			} else {
				log.Println(err)
				c.AbortWithStatus(http.StatusInternalServerError)
			}
			return
		}
		baseUrl, err := service.GetConfig("base_url")
		if err != nil {
			log.Println(err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		liveM3U8, err := service.GetYoutubeLiveM3U8(channelInfo.URL)
		if err != nil {
			log.Println(err)
			// c.AbortWithStatus(http.StatusInternalServerError)
			// return a placeholder video
			m3u8Body = "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:10\n#EXT-X-MEDIA-SEQUENCE:0\n#EXTINF:10.000000,\n" + baseUrl + "/placeholder.ts\n#EXT-X-ENDLIST" // make a fake m3u8 pointing to the target
		} else {
			client := http.Client{Timeout: global.HttpClientTimeout}
			resp, err := client.Get(liveM3U8)
			if err != nil {
				log.Println(err)
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}

			bodyString := ""
			defer resp.Body.Close()
			if strings.Contains(resp.Header.Get("Content-Type"), "mpegurl") {
				bodyBytes, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					log.Println(err)
					c.AbortWithStatus(http.StatusInternalServerError)
					return
				}
				bodyString = string(bodyBytes)
			} else {
				service.UpdateStatus(channelInfo.URL, service.Warning, "Url is not a live stream")
				bodyString = "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:999\n#EXT-X-MEDIA-SEQUENCE:0\n#EXTINF:999.000000, video\n" + liveM3U8 + "\n#EXT-X-ENDLIST"// make a fake m3u8 pointing to the target
			}
			if channelInfo.Proxy {
				m3u8Body = service.M3U8Process(bodyString, baseUrl+"/live.ts?k=")
			} else {
				m3u8Body = bodyString
			}
			global.M3U8Cache.Set(channelCacheKey, m3u8Body, 3*time.Second)
		}
	}
	c.Data(http.StatusOK, "application/vnd.apple.mpegurl", []byte(m3u8Body))
}

func TsProxyHandler(c *gin.Context) {
	zipedRemoteURL := c.Query("k")
	remoteURL, err := util.DecompressString(zipedRemoteURL)
	if err != nil {
		log.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if remoteURL == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	client := http.Client{Timeout: global.HttpClientTimeout}
	resp, err := client.Get(remoteURL)
	if err != nil {
		log.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	c.DataFromReader(http.StatusOK, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, nil)
}

func CacheHandler(c *gin.Context) {
	var sb strings.Builder
	global.URLCache.Range(func(k, v interface{}) bool {
		sb.WriteString(k.(string))
		sb.WriteString(" => ")
		sb.WriteString(v.(string))
		sb.WriteString("\n")
		return true
	})
	c.Data(http.StatusOK, "text/plain", []byte(sb.String()))
}
