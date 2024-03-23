// plugins
package plugin

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/grafov/m3u8"
)

type Plugin interface {
	Parse(liveUrl string) (url string, logo string, error error)
}

var (
	pluginCenter  map[string]Plugin = make(map[string]Plugin)
	NoMatchPlugin error             = errors.New("No matching plugin found")
	NoMatchFeed   error             = errors.New("This channel is not currently live")
)

const (
	DefaultUserAgent string = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
)

func registerPlugin(name string, parser Plugin) {
	pluginCenter[name] = parser
}

func getBaseURL(rawURL string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	// Remove the last element (document) from the path
	parsedURL.Path = path.Dir(parsedURL.Path) + "/"

	// Rebuild the URL without the document part
	return parsedURL.String()
}

func isValidURL(u string) bool {
	_, err := url.ParseRequestURI(u)
	return err == nil
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
