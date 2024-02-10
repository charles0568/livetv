package service

import (
	"context"
	"log"
	"os/exec"
	"strings"

	"github.com/zjyl1994/livetv/global"
)

func GetYoutubeLiveM3U8(youtubeURL string) (string, error) {
	liveURL, ok := global.URLCache.Load(youtubeURL)
	if ok {
		return liveURL.(string), nil
	} else {
		log.Println("cache miss", youtubeURL)
		liveURL, err := RealGetYoutubeLiveM3U8(youtubeURL)
		if err != nil {
			log.Println(err)
			log.Println("[YTDL]", liveURL)
			return "", err
		} else {
			global.URLCache.Store(youtubeURL, liveURL)
			return liveURL, nil
		}
	}
}

func RealGetYoutubeLiveM3U8(youtubeURL string) (string, error) {
	YtdlCmd, err := GetConfig("ytdl_cmd")
	if err != nil {
		log.Println(err)
		UpdateStatus(youtubeURL, Error, err.Error())
		return "", err
	}
	YtdlArgs, err := GetConfig("ytdl_args")
	if err != nil {
		log.Println(err)
		UpdateStatus(youtubeURL, Error, err.Error())
		return "", err
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
		UpdateStatus(youtubeURL, Error, err.Error())
		return "", err
	} else {
		ctx, cancelFunc := context.WithTimeout(context.Background(), global.HttpClientTimeout)
		defer cancelFunc()
		cmd := exec.CommandContext(ctx, YtdlCmd, ytdlArgs...)
		out, err := cmd.CombinedOutput()
		output := strings.TrimSpace(string(out))
		if err == nil {
			UpdateStatus(youtubeURL, Ok, "Ready")
		} else {
			UpdateStatus(youtubeURL, Error, output)
		}
		return output, err
	}
}
