package handler

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/zjyl1994/livetv/model"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"github.com/zjyl1994/livetv/global"
	"github.com/zjyl1994/livetv/service"
	"github.com/zjyl1994/livetv/util"
)

func M3UHandler(c *gin.Context) {
	disableProtection := os.Getenv("LIVETV_FREEACCESS") == "1"
	// verify token against the unique token of the requested channel
	if !disableProtection {
		token := c.Query("token")
		if token != global.GetSecretToken() { // invalid token
			c.String(http.StatusForbidden, "Forbidden")
			return
		}
	}

	content, err := service.M3UGenerate()
	if err != nil {
		log.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.Data(http.StatusOK, "application/vnd.apple.mpegurl", []byte(content))
}

func LivePreHandler(c *gin.Context) {
	channelNumber := util.String2Uint(c.Query("c"))
	if channelNumber == 0 {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	_, err := service.GetChannel(channelNumber)
	if err != nil {
		if gorm.IsRecordNotFoundError(err) {
			c.AbortWithStatus(http.StatusNotFound)
		} else {
			log.Println(err)
			c.AbortWithStatus(http.StatusInternalServerError)
		}
		return
	}
	c.Data(http.StatusOK, "application/vnd.apple.mpegurl", []byte(nil))
}

func handleNonHTTPProtocol(m3u8url string, c *gin.Context) (handled bool) {
	handled = false
	u, err := url.Parse(m3u8url)
	if err == nil && !strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Scheme, "https") {
		c.Redirect(http.StatusFound, m3u8url)
		handled = true
	}
	return
}

func LiveHandler(c *gin.Context) {
	channelCacheKey := c.Query("c")
	disableProtection := os.Getenv("LIVETV_FREEACCESS") == "1"
	// verify token against the unique token of the requested channel
	if !disableProtection {
		token := c.Query("token")
		channelNumber, err := strconv.Atoi(channelCacheKey)
		if err != nil { // invalid channel id format
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
		ch, err := service.GetChannel(uint(channelNumber))
		if err != nil { // non-existent channel
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
		if token != ch.Token { // invalid token
			c.String(http.StatusForbidden, "Forbidden")
			return
		}
	}

	var m3u8Body string
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
		baseUrl, err := global.GetConfig("base_url")
		if err != nil {
			log.Println(err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		liveM3U8, _, err := service.GetLiveM3U8(channelInfo.URL, channelInfo.Parser)
		if err != nil {
			log.Println(err)
			// c.AbortWithStatus(http.StatusInternalServerError)
			// return a placeholder video
			m3u8Body = service.PlaceHolderHLS() // make a fake m3u8 pointing to the target
		} else {
			// handle non http protocols like rtsp, rtmp and etc.
			if handleNonHTTPProtocol(liveM3U8, c) {
				return
			}
			client := http.Client{Timeout: global.HttpClientTimeout}
			req, err := http.NewRequest(http.MethodGet, liveM3U8, nil)
			if err != nil {
				log.Println(err)
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			req.Header.Set("User-Agent", service.DefaultUserAgent)
			resp, err := client.Do(req)
			if err != nil {
				log.Println(err)
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}

			bodyString := ""
			defer resp.Body.Close()
			if resp.ContentLength < 10*1024*1024 && strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "mpegurl") {
				bodyBytes, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					log.Println(err)
					c.AbortWithStatus(http.StatusInternalServerError)
					return
				}
				bodyString = string(bodyBytes)
			} else {
				service.UpdateStatus(channelInfo.URL, service.Warning, "Url is not a live stream")
				duration, err := service.GetVideoDuration(channelInfo.URL)
				if err == nil && duration > 0 {
					log.Println(channelInfo.URL, "duration is", duration)
					bodyString = fmt.Sprintf("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:%.0f\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXT-X-MEDIA-SEQUENCE:0\n#EXTINF:%.4f, video\n%s\n#EXT-X-ENDLIST", duration, duration, liveM3U8)
				} else {
					log.Println("failed to get duration", err.Error())
					bodyString = "#EXTM3U\n#EXTINF:-1, video\n#EXT-X-PLAYLIST-TYPE:VOD\n" + liveM3U8 + "\n#EXT-X-ENDLIST" // make a fake m3u8 pointing to the target
				}
			}
			m3u8Body = service.M3U8Process(liveM3U8, bodyString, baseUrl+"/live.ts?token="+global.GetLiveToken()+"&k=", channelInfo.Proxy)
			// if channelInfo.Proxy {
			// 	m3u8Body = service.M3U8Process(liveM3U8, bodyString, baseUrl+"/live.ts?k=")
			// } else {
			// 	m3u8Body = bodyString
			// }
			global.M3U8Cache.Set(channelCacheKey, m3u8Body, 3*time.Second)
		}
	}
	c.Data(http.StatusOK, "application/vnd.apple.mpegurl", []byte(m3u8Body))
}

func TsProxyHandler(c *gin.Context) {
	// verify access token if protection is enabled (by default)
	disableProtection := os.Getenv("LIVETV_FREEACCESS") == "1"
	if !disableProtection {
		token := c.Query("token")
		if token != global.GetLiveToken() {
			c.String(http.StatusForbidden, "Forbidden")
			return
		}
	}

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
	rurl, err := url.Parse(remoteURL)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
	}

	client := http.Client{Timeout: global.HttpClientTimeout}
	req := c.Request.Clone(context.Background())
	req.RequestURI = ""
	req.Host = ""
	req.URL = rurl
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	for key, values := range resp.Header {
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}
	defer resp.Body.Close()
	c.Writer.WriteHeader(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}

func CacheHandler(c *gin.Context) {
	var sb strings.Builder
	global.URLCache.Range(func(k string, v *model.LiveInfo) bool {
		sb.WriteString(k)
		sb.WriteString(" => ")
		sb.WriteString(v.LiveUrl)
		sb.WriteString("\n")
		return true
	})
	c.Data(http.StatusOK, "text/plain", []byte(sb.String()))
}
