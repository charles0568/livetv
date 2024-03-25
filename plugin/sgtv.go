// sgtv
package plugin

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/zjyl1994/livetv/model"
)

var sgtvAESKey []byte = []byte("ilyB29ZdruuQjC45JhBBR7o2Z8WJ26Vg")

// the current channel API endpoint is https://api2.4gtv.tv/Channel/GetChannelUrl3

type SGTVParser struct{}

type IdentityValidate struct {
	Value string `json:"fsVALUE"`
}

type SGTVRequest struct {
	ChannelID        string `json:"fnCHANNEL_ID"`
	AssetID          string `json:"fsASSET_ID"`
	DeviceType       string `json:"fsDEVICE_TYPE"`
	IdentityValidate IdentityValidate
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
	ciphertext := make([]byte, aes.BlockSize+len(input))
	copy(ciphertext[:aes.BlockSize], iv)

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext[aes.BlockSize:], input)

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
	plaintext := unpad(ciphertext[aes.BlockSize:])

	return plaintext, nil
}

func unpad(src []byte) []byte {
	length := len(src)
	unpadding := int(src[length-1])

	return src[:(length - unpadding)]
}

func generateIV() ([]byte, error) {
	iv := make([]byte, 16) // AES block size is 16 bytes
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	return iv, nil
}

func (p *SGTVParser) Parse(liveUrl string, lastInfo string) (*model.LiveInfo, error) {
	iv, _ := generateIV()
	u, _ := url.Parse(liveUrl)
	var sgtvReq SGTVRequest
	sgtvReq.ChannelID = u.Query().Get("ChannelID")
	sgtvReq.AssetID = u.Query().Get("AssetID")
	sgtvReq.DeviceType = "pc"

	if sgtvReq.ChannelID == "" || sgtvReq.AssetID == "" {
		return nil, errors.New("Channel and asset ID must be provided")
	}

	u.RawQuery = "" // drop our custom made querystring

	client := http.Client{
		Timeout: time.Second * 10,
	}
	body, _ := json.Marshal(&sgtvReq)
	log.Println("json request is", body)
	encodedBody, _ := p.encrypt(body, iv) // encrypt our request
	log.Println("encrypted", encodedBody)
	formData := url.Values{"value": {encodedBody}}
	log.Println("post body", formData.Encode())
	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader([]byte(formData.Encode())))
	req.Header.Set("User-Agent", DefaultUserAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	// DO not parse invalid response, parse json only
	if resp.ContentLength > 10*1024*1024 || !strings.Contains(resp.Header.Get("Content-Type"), "json") {
		return nil, errors.New("invalid response")
	}
	content, _ := io.ReadAll(resp.Body)
	log.Println("response:", content)
	var sgtvResp SGTVResponse
	json.Unmarshal(content, &sgtvResp)
	if sgtvResp.Success {
		cleartext, err := p.decrypt(sgtvResp.Data, iv)
		if err == nil {
			var chInfo SGTVChannelInfo
			json.Unmarshal(cleartext, &chInfo)
			log.Println(chInfo)
		}
	}
	return nil, NoMatchFeed
}

func init() {
	registerPlugin("4GTV", &SGTVParser{})
}
