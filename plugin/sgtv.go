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

func (p *SGTVParser) Parse(liveUrl string, proxyUrl string, lastInfo string) (*model.LiveInfo, error) {
	iv := sgtvIV // yes, it's predefined and fully static
	u, urlerr := url.Parse(liveUrl)
	if urlerr != nil {
		return nil, urlerr
	}
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
	resp, err := cloudScraper(req, proxyUrl)
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
				//log.Println("master playlist", chInfo.Urls[0])
				liveUrl, err := bestFromMasterPlaylist(chInfo.Urls[0], proxyUrl) // extract the best quality live url from the master playlist
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
