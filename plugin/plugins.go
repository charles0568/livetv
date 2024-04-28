// plugins
package plugin

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/zjyl1994/livetv/global"

	"github.com/dlclark/regexp2"
	"github.com/zjyl1994/livetv/model"

	"github.com/grafov/m3u8"
)

type Plugin interface {
	Parse(liveUrl string, lastInfo string) (info *model.LiveInfo, error error)
}

type Transformer interface {
	Transform(playList string, lastInfo string) (string, error)
}

type HealthCheck interface {
	Check(content string, info *model.LiveInfo) error
}

var (
	pluginCenter  map[string]Plugin = make(map[string]Plugin)
	NoMatchPlugin error             = errors.New("No matching plugin found")
	NoMatchFeed   error             = errors.New("This channel is not currently live")
)

const (
	DefaultUserAgent string = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

func registerPlugin(name string, parser Plugin) {
	pluginCenter[name] = parser
}

func bestFromMasterPlaylist(masterUrl string, content ...io.Reader) (string, error) {
	var playlist io.Reader
	if len(content) > 0 {
		playlist = content[0]
	} else {
		client := http.Client{
			Timeout: time.Second * 10,
		}
		req, err := http.NewRequest("GET", masterUrl, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", DefaultUserAgent)
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.ContentLength > 10*1024*1024 || !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "mpegurl") {
			return "", errors.New("invalid url")
		}
		playlist = resp.Body
	}
	p, listType, err := m3u8.DecodeFrom(playlist, true)
	// log.Println("parsed playlist", p == nil, listType, err)
	if p == nil {
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
			if !global.IsValidURL(selectedUrl) {
				selectedUrl = global.GetBaseURL(masterUrl) + selectedUrl
			}
			return selectedUrl, nil
		}
	}
	return "", errors.New("Unknown type of playlist")
}

// regex from https://stackoverflow.com/questions/5830387/how-do-i-find-all-youtube-video-ids-in-a-string-using-a-regex?lq=1
func getYouTubeVideoID(url string) string {
	regex := regexp2.MustCompile(`(?:youtu\.be\/|youtube(?:-nocookie)?\.com\S*?[^\w\s-])([\w-]{11})(?=[^\w-]|$)(?![?=&+%\w.-]*(?:['"][^<>]*>|<\/a>))[?=&+%\w.-]*`, 0)
	match, _ := regex.FindStringMatch(url)
	if match != nil && len(match.Groups()) > 0 {
		return match.Groups()[0].Captures[0].String()
	}
	return ""
}

func getYouTubeChannelID(url string) string {
	regex := regexp2.MustCompile(`youtu((\.be)|(be\..{2,5}))\/((user)|(channel)|(c)|(@))\/?([a-zA-Z0-9\-_]{1,})`, 0)
	match, _ := regex.FindStringMatch(url)
	if match != nil && len(match.Groups()) > 0 {
		return match.Groups()[9].Captures[0].String()
	}
	return ""
}

func GetPlugin(name string) (Plugin, error) {
	if p, ok := pluginCenter[name]; ok {
		return p, nil
	}
	return nil, NoMatchPlugin
}

func GetPluginList() []string {
	list := make([]string, 0)
	for name, _ := range pluginCenter {
		list = append(list, name)
	}
	return list
}
