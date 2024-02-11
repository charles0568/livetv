package service

import (
	"context"
	"errors"
	"regexp"

	"github.com/sosodev/duration"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

func getYouTubeID(url string) string {
	regex := regexp.MustCompile(`(?:youtube(?:-nocookie)?\.com\/(?:[^\/\n\s]+\/\S+\/|(?:v|e(?:mbed)?)\/|\S*?[?&]v=)|youtu\.be\/)([a-zA-Z0-9_-]{11})`)
	match := regex.FindStringSubmatch(url)
	if match != nil && len(match[1]) == 11 {
		return match[1]
	}
	return ""
}

func GetVideoDuration(url string) (float64, error) {
	vid := getYouTubeID(url)
	if vid == "" {
		return 0, errors.New("not a valid video url")
	}
	apiKey, err := GetConfig("apiKey")
	if err != nil {
		return 0, err
	}
	if apiKey == "" {
		return 0, errors.New("API not set")
	}
	ctx := context.Background()
	service, err := youtube.NewService(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return 0, err
	}

	// Call the videos.list method with the video ID
	resp, err := service.Videos.List([]string{"contentDetails"}).Id(vid).Do()
	if err == nil && len(resp.Items) > 0 {
		d, err := duration.Parse(resp.Items[0].ContentDetails.Duration)
		if err == nil {
			return d.ToTimeDuration().Seconds(), nil
		}
	}
	return 0, err
}
