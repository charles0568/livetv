package service

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/dlclark/regexp2"
	"github.com/grafov/m3u8"
	"github.com/zjyl1994/livetv/global"
	"github.com/zjyl1994/livetv/util"
)

var errNoMatchFound error = errors.New("This channel is not currently live")

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
	} else if time.Now().Add(time.Second * 180).After(expireTime) {
		go UpdateURLCacheSingle(youtubeURL) // update async
	}
	return false
}

func GetYoutubeLiveM3U8(youtubeURL string) (string, string, error) {
	liveURL, ok := global.URLCache.Load(youtubeURL)
	logo, _ := global.LogoCache.Load(youtubeURL)
	if ok {
		// check and refresh expired/expiring feed
		if checkAndUpdateExpiringM3U8(youtubeURL, liveURL) {
			// expired link, should load liveUrl again
			liveURL, ok = global.URLCache.Load(youtubeURL)
			if !ok {
				return "", "", errNoMatchFound
			}
		}
		return liveURL, logo, nil
	} else {
		log.Println("cache miss", youtubeURL)
		status := GetStatus(youtubeURL)
		if time.Now().Sub(status.Time) > time.Minute*2 {
			return UpdateURLCacheSingle(youtubeURL)
		} else {
			return "", "", errors.New("parser cooling down")
		}
	}
}

func bestFromMasterPlaylist(masterUrl string) (string, error) {
	client := http.Client{
		Timeout: time.Second * 10,
	}
	resp, err := client.Get(masterUrl)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.ContentLength > 10*1024*1024 || !strings.Contains(resp.Header.Get("Content-Type"), "mpegurl") {
		return "", errors.New("invalid url")
	}
	p, listType, err := m3u8.DecodeFrom(resp.Body, true)
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

// inspired by https://github.com/abskmj/youtube-hls-m3u8
func DoGetYoutubeLiveM3U8Internal(youtubeURL string) (string, string, error) {
	client := http.Client{
		Timeout: time.Second * 10,
	}
	resp, err := client.Get(youtubeURL)
	if err != nil {
		return "", "", err
	}

	defer resp.Body.Close()
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
	return "", "", errNoMatchFound
}

func DoGetYoutubeLiveM3U8External(youtubeURL string) (string, string, error) {
	YtdlCmd, err := GetConfig("ytdl_cmd")
	if err != nil {
		log.Println(err)
		return "", "", err
	}
	YtdlArgs, err := GetConfig("ytdl_args")
	if err != nil {
		log.Println(err)
		return "", "", err
	}
	ytdlArgs := strings.Fields(YtdlArgs)
	for i, v := range ytdlArgs {
		if strings.EqualFold(v, "{url}") {
			ytdlArgs[i] = youtubeURL
		}
	}
	_, err = exec.LookPath(YtdlCmd)
	if err != nil {
		log.Println(err)
		return "", "", err
	} else {
		ctx, cancelFunc := context.WithTimeout(context.Background(), global.HttpClientTimeout)
		defer cancelFunc()
		cmd := exec.CommandContext(ctx, YtdlCmd, ytdlArgs...)
		out, err := cmd.CombinedOutput()
		output := strings.TrimSpace(string(out))
		if err == nil {
			return output, "", err
		} else {
			if output == "" {
				return "", "", err
			} else {
				return "", "", errors.Join(errors.New(output+" , "), err)
			}
		}
	}
}

func RealGetYoutubeLiveM3U8(youtubeURL string) (string, string, error) {
	if feed, logo, err := DoGetYoutubeLiveM3U8Internal(youtubeURL); err != nil && !errors.Is(err, errNoMatchFound) {
		log.Println("Internal resolver returned with error", err)
		return DoGetYoutubeLiveM3U8External(youtubeURL)
	} else {
		return feed, logo, err
	}
}
