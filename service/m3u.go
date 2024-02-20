package service

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/zjyl1994/livetv/global"
)

func M3UGenerate() (string, error) {
	baseUrl, err := GetConfig("base_url")
	if err != nil {
		log.Println(err)
		return "", err
	}
	channels, err := GetAllChannel()
	if err != nil {
		log.Println(err)
		return "", err
	}
	var m3u strings.Builder
	m3u.WriteString("#EXTM3U\n")
	for _, v := range channels {
		logo := ""
		if _logo, ok := global.LogoCache.Load(v.URL); ok {
			logo = _logo
		}
		liveData := fmt.Sprintf("#EXTINF:-1, tvg-name=%s tvg-logo=%s group-title=\"LiveTV\", %s\n", strconv.Quote(v.Name), strconv.Quote(logo), v.Name)
		m3u.WriteString(liveData)
		m3u.WriteString(baseUrl)
		m3u.WriteString("/live.m3u8?c=")
		m3u.WriteString(strconv.Itoa(int(v.ID)))
		m3u.WriteString("\n")
	}
	return m3u.String(), nil
}
