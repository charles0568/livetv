// youtube
package plugin

import (
	"context"
	"errors"
	"log"
	"os/exec"
	"strings"

	"github.com/zjyl1994/livetv/global"
)

type YtDlpParser struct{}

func (p *YtDlpParser) Parse(liveUrl string) (string, string, error) {
	YtdlCmd, err := global.GetConfig("ytdl_cmd")
	if err != nil {
		log.Println(err)
		return "", "", err
	}
	YtdlArgs, err := global.GetConfig("ytdl_args")
	if err != nil {
		log.Println(err)
		return "", "", err
	}
	ytdlArgs := strings.Fields(YtdlArgs)
	for i, v := range ytdlArgs {
		if strings.EqualFold(v, "{url}") {
			ytdlArgs[i] = liveUrl
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

func init() {
	registerPlugin("yt-dlp", &YtDlpParser{})
}
