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
	"net/http"
	"net/url"
	"path/filepath"

	freq "github.com/imroc/req/v3"
	"github.com/zjyl1994/livetv/model"
)

var (
	sgtvAESKey []byte = []byte("ilyB29ZdruuQjC45JhBBR7o2Z8WJ26Vg")
	sgtvIV     []byte = []byte("JUMxvVMmszqUTeKn")
	sgtvAPI    string = "https://api2.4gtv.tv/Channel/GetChannelUrl3"
)

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

func cloudScraper(req *http.Request) (*freq.Response, error) {
	client := freq.ImpersonateChrome().SetCommonContentType("application/x-www-form-urlencoded; charset=UTF-8").SetCommonHeader("accept", "*/*")

	return client.R().SetBody(req.Body).Post(req.URL.String())

	// // Client also will need a cookie jar.
	// // client := http.Client{}
	// // cookieJar, _ := cookiejar.New(nil)
	// // client.Jar = cookieJar
	// client, _ := newTlsClient()
	// req.Header = http.Header{
	// 	// "sec-ch-ua":        {`"Chromium";v="122", "Not(A:Brand";v="24", "Google Chrome";v="122"`},
	// 	"sec-ch-ua-mobile":   {`?1`},
	// 	"User-Agent":         {`Mozilla/5.0 (iPad; CPU OS 16_7 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) EdgiOS/121.0.2277.107 Version/16.0 Mobile/15E148 Safari/604.1`},
	// 	"Accept":             {`*/*`},
	// 	"Sec-Fetch-Site":     {`same-site`},
	// 	"Sec-Fetch-Mode":     {`cors`},
	// 	"Sec-Fetch-Dest":     {`empty`},
	// 	"Content-Type":       {"application/x-www-form-urlencoded; charset=UTF-8"},
	// 	"Accept-Encoding":    {`gzip, deflate`},
	// 	"Accept-Language":    {`en-US,en;q=0.9`},
	// 	http.HeaderOrderKey:  {"sec-ch-ua", "sec-ch-ua-mobile", "upgrade-insecure-requests", "user-agent", "accept", "sec-fetch-site", "sec-fetch-mode", "sec-fetch-user", "sec-fetch-dest", "accept-encoding", "accept-language"},
	// 	http.PHeaderOrderKey: {":method", ":authority", ":scheme", ":path"},
	// }

	// return client.Do(req)
}

func (p *SGTVParser) Parse(liveUrl string, lastInfo string) (*model.LiveInfo, error) {
	iv := sgtvIV // yes, it's predefined and fully static
	u, _ := url.Parse(liveUrl)
	var sgtvReq SGTVRequest
	sgtvReq.ChannelID = u.Query().Get("ch")
	sgtvReq.AssetID = filepath.Base(u.Path)
	sgtvReq.DeviceType = "mobile"

	if sgtvReq.ChannelID == "" || sgtvReq.AssetID == "" {
		return nil, errors.New("Channel and asset ID must be provided")
	}

	body, _ := json.Marshal(&sgtvReq)
	encodedBody, _ := p.encrypt(body, iv) // encrypt our request
	formData := url.Values{"value": {encodedBody}}
	req, err := http.NewRequest(http.MethodPost, sgtvAPI, bytes.NewReader([]byte(formData.Encode())))
	if err != nil {
		return nil, err
	}
	resp, err := cloudScraper(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	content, _ := io.ReadAll(resp.Body)
	log.Println("response:", string(content))
	var sgtvResp SGTVResponse
	json.Unmarshal(content, &sgtvResp)
	if sgtvResp.Success {
		cleartext, err := p.decrypt(sgtvResp.Data, iv)
		if err == nil {
			var chInfo SGTVChannelInfo
			if json.Unmarshal(cleartext, &chInfo) == nil && len(chInfo.Urls) > 0 {
				liveUrl, err := bestFromMasterPlaylist(chInfo.Urls[0]) // extract the best quality live url from the master playlist
				if err != nil {
					return nil, err
				}
				li := &model.LiveInfo{}
				li.LiveUrl = liveUrl
				li.Logo = chInfo.Cover
				return li, nil
			}
		}
	}
	return nil, NoMatchFeed
}

func init() {
	registerPlugin("4GTV", &SGTVParser{})
}
