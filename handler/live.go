package handler

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/zjyl1994/livetv/plugin"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"github.com/zjyl1994/livetv/global"
	"github.com/zjyl1994/livetv/model"
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
		proxyUrl := channelInfo.TsProxy
		if proxyUrl == "" {
			proxyUrl = baseUrl
		}
		liveInfo, err := service.GetLiveM3U8(channelInfo.URL, channelInfo.ProxyUrl, channelInfo.Parser)
		if err != nil {
			log.Println(err)
			// return a placeholder video
			m3u8Body = service.PlaceHolderHLS() // make a fake m3u8 pointing to the target
		} else {
			parser, err := plugin.GetPlugin(channelInfo.Parser)
			if err == nil {
				if handler, ok := parser.(plugin.FeedHost); ok {
					// handler has the ability host the feed and succeeded
					if handler.Host(c, liveInfo) == nil {
						return
					}
				}
			}
			// handle non http protocols like rtsp, rtmp and etc.
			if handleNonHTTPProtocol(liveInfo.LiveUrl, c) {
				return
			}
			// the GetM3U8Content will handle health-check, reparse, url decoration etc. and returns the final result and the final url used
			bodyString, finalUrl, err := service.GetM3U8Content(channelInfo.URL, liveInfo.LiveUrl, channelInfo.ProxyUrl, channelInfo.Parser)
			if bodyString == "" {
				log.Println(err)
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			iTsTransformer, _ := parser.(plugin.TsTransformer)
			// get m3u8 content and transcode into tsproxy link if needed
			m3u8Body = service.M3U8Process(finalUrl, bodyString, proxyUrl+"/live.ts?token="+global.GetLiveToken()+"&k=", channelInfo.Proxy,
				func(raw string, ts string) string {
					if iTsTransformer == nil {
						return ts
					}
					return iTsTransformer.TransformTs(raw, ts, liveInfo) // allow plugins to override our default tslink
				})
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
	// added possible custom headers
	queries := c.Request.URL.Query()
	for key, value := range queries {
		if strings.HasPrefix(key, "header") && len(value) > 0 {
			req.Header.Add(key[6:], value[0])
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err, req, resp)
		resp, _ := client.Transport.RoundTrip(req)
		log.Println(resp)
		c.AbortWithStatus(http.StatusInternalServerError)
		c.Writer.WriteString(err.Error())
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
