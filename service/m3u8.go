package service

import (
	"bufio"
	"fmt"
	"strings"
	"time"

	"github.com/zjyl1994/livetv/util"
)

func PlaceHolderHLS() string {
	t := time.Now().Unix() / 60 % 100000
	baseUrl, _ := GetConfig("base_url")
	if !strings.HasSuffix(baseUrl, "/") {
		baseUrl += "/"
	}
	placeholder := baseUrl + "placeholder.ts"
	tpl := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-MEDIA-SEQUENCE:%d
#EXT-X-TARGETDURATION:60
#EXT-X-DISCONTINUITY:0
#EXTINF:60.000000,
%s?t=%d
#EXTINF:60.000000,
%s?t=%d
#EXTINF:60.000000,
%s?t=%d
`
	return fmt.Sprintf(tpl, t%10000, placeholder, (t-1)%10000, placeholder, t%10000, placeholder, (t+1)%10000)
}

func M3U8Process(data string, prefixURL string) string {
	var sb strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		l := scanner.Text()
		if strings.HasPrefix(l, "#") {
			sb.WriteString(l)
		} else {
			sb.WriteString(prefixURL)
			sb.WriteString(util.CompressString(l))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
