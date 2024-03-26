// sgtv
package plugin

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/url"

	hawk "github.com/juiced-aio/hawk-go"
	http "github.com/useflyent/fhttp"
	"github.com/useflyent/fhttp/cookiejar"
	"github.com/zjyl1994/livetv/model"
)

var sgtvAESKey []byte = []byte("ilyB29ZdruuQjC45JhBBR7o2Z8WJ26Vg")
var sgtvIV []byte = []byte("JUMxvVMmszqUTeKn")

// the current channel API endpoint is https://api2.4gtv.tv/Channel/GetChannelUrl3

type SGTVParser struct{}

type IdentityValidate struct {
	Value string `json:"fsVALUE"`
}

type SGTVRequest struct {
	ChannelID        string           `json:"fnCHANNEL_ID"`
	AssetID          string           `json:"fsASSET_ID"`
	DeviceType       string           `json:"fsDEVICE_TYPE"`
	IdentityValidate IdentityValidate `json:"clsIDENTITY_VALIDATE_ARUS"`
}

type SGTVResponse struct {
	Data       string
	Success    bool
	Status     int32
	ErrMessage string
}

type SGTVChannelInfo struct {
	ChannelName string   `json:"fsCHANNEL_NAME"`
	Urls        []string `json:"flstURLs"`
	Cover       string   `json:"fsHEAD_FRAME"`
	BitRate     []int32  `json:"flstBITRATE"`
}

func (p *SGTVParser) encrypt(input []byte, iv []byte) (string, error) {
	block, err := aes.NewCipher(sgtvAESKey)
	if err != nil {
		return "", err
	}

	// The IV needs to be unique, but not secure. Therefore it's common to
	// include it at the beginning of the ciphertext.
	paddedInput := pad([]byte(input), aes.BlockSize)
	ciphertext := make([]byte, len(paddedInput))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, paddedInput)

	// Convert to base64
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (p *SGTVParser) decrypt(response string, iv []byte) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(response)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}

	// CBC mode always works in whole blocks.
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, errors.New("ciphertext is not a multiple of the block size")
	}

	block, err := aes.NewCipher(sgtvAESKey)
	if err != nil {
		return nil, err
	}

	mode := cipher.NewCBCDecrypter(block, iv)

	// CryptBlocks can work in-place if the two arguments are the same.
	mode.CryptBlocks(ciphertext, ciphertext)

	// Unpad the input to retrieve the original plaintext
	plaintext := unpad(ciphertext)

	return plaintext, nil
}

// pad applies PKCS#7 padding to the input.
func pad(input []byte, blockSize int) []byte {
	padding := blockSize - len(input)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(input, padtext...)
}

func unpad(src []byte) []byte {
	length := len(src)
	unpadding := int(src[length-1])

	return src[:(length - unpadding)]
}

func cloudScraper(req *http.Request) (*http.Response, error) {
	// Client also will need a cookie jar.
	client := http.Client{}
	cookieJar, _ := cookiejar.New(nil)
	client.Jar = cookieJar
	scraper := hawk.CFInit(client, "YOUR_KEY_HERE", true)
	// You will have to create your own function if you want to solve captchas.
	scraper.CaptchaFunction = func(originalURL string, siteKey string) (string, error) {
		// CaptchaFunction should return the token as a string.
		return "", nil
	}
	req.Header = http.Header{
		"sec-ch-ua":                 {`"Chromium";v="92", " Not A;Brand";v="99", "Google Chrome";v="92"`},
		"sec-ch-ua-mobile":          {`?0`},
		"upgrade-insecure-requests": {`1`},
		"user-agent":                {`Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/92.0.4515.107 Safari/537.36`},
		"accept":                    {`text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9`},
		"sec-fetch-site":            {`none`},
		"sec-fetch-mode":            {`navigate`},
		"sec-fetch-user":            {`?1`},
		"sec-fetch-dest":            {`document`},
		"content-type":              {"application/x-www-form-urlencoded; charset=UTF-8"},
		"accept-encoding":           {`gzip, deflate`},
		"accept-language":           {`en-US,en;q=0.9`},
		http.HeaderOrderKey:         {"sec-ch-ua", "sec-ch-ua-mobile", "upgrade-insecure-requests", "user-agent", "accept", "sec-fetch-site", "sec-fetch-mode", "sec-fetch-user", "sec-fetch-dest", "accept-encoding", "accept-language"},
		http.PHeaderOrderKey:        {":method", ":authority", ":scheme", ":path"},
	}

	return scraper.Do(req)
}

func (p *SGTVParser) Parse(liveUrl string, lastInfo string) (*model.LiveInfo, error) {
	iv := sgtvIV // yes, it's predefined and fully static
	u, _ := url.Parse(liveUrl)
	var sgtvReq SGTVRequest
	sgtvReq.ChannelID = u.Query().Get("ChannelID")
	sgtvReq.AssetID = u.Query().Get("AssetID")
	sgtvReq.DeviceType = "pc"

	if sgtvReq.ChannelID == "" || sgtvReq.AssetID == "" {
		return nil, errors.New("Channel and asset ID must be provided")
	}

	u.RawQuery = "" // drop our custom made querystring

	body, _ := json.Marshal(&sgtvReq)
	log.Println("json request is", string(body))
	encodedBody, _ := p.encrypt(body, iv) // encrypt our request
	log.Println("encrypted", encodedBody)
	formData := url.Values{"value": {encodedBody}}
	log.Println("post body", formData.Encode())
	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader([]byte(formData.Encode())))
	req.Header.Set("User-Agent", DefaultUserAgent)
	req.Header.Set("Accept-Language", "en,en-US;q=0.9,zh-CN;q=0.8,zh;q=0.7,zh-TW;q=0.6")
	// req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Referer", "https://www.4gtv.tv/")
	req.Header.Set("sec-ch-ua", "\"Chromium\";v=\"120\", \"Not(A:Brand\";v=\"24\", \"Google Chrome\";v=\"120\"")
	req.Header.Set("DNT", "1")
	req.Header.Set("Origin", "https://www.4gtv.tv")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("sec-ch-ua-platform", "\"Windows\"")
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("Sec-Fetch-Site", "same-site")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	resp, err := cloudScraper(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	content, _ := io.ReadAll(resp.Body)
	log.Println(string(content))
	var sgtvResp SGTVResponse
	json.Unmarshal(content, &sgtvResp)
	if sgtvResp.Success {
		cleartext, err := p.decrypt(sgtvResp.Data, iv)
		if err == nil {
			var chInfo SGTVChannelInfo
			if json.Unmarshal(cleartext, &chInfo) == nil && len(chInfo.Urls) > 0 {
				li := &model.LiveInfo{}
				li.LiveUrl = chInfo.Urls[0]
				return li, nil
			}
		}
	}
	return nil, NoMatchFeed
}

func init() {
	registerPlugin("4GTV", &SGTVParser{})
}
