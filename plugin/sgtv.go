// sgtv
package plugin

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

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

func fakeChromeRequest(req *http.Request) (*http.Response, error) {
	conn, err := tls.Dial("tcp", req.Host+":443", &tls.Config{
		ServerName: req.Host,
	})

	// conn, err := net.Dial("tcp", req.Host+":80")
	if err == nil {
		defer conn.Close()
	} else {
		return nil, err
	}
	fakeRequestTemplate := "POST %s HTTP/1.1\r\n" +
		"Host: %s\r\nUser-Agent: %s\r\n" +
		"Content-Type: application/x-www-form-urlencoded; charset=UTF-8\r\n" +
		"Accept: */*\r\n" +
		"Accept-Language: en,en-US;q=0.9,zh-CN;q=0.8,zh;q=0.7,zh-TW;q=0.6\r\n" +
		"Connection: keep-alive\r\n" +
		"Content-Length: %d\r\n" +
		"Referrer: https://www.4gtv.tv/\r\n" +
		"ReferrerPolicy: strict-origin-when-cross-origin\r\n" +
		"\r\n%s"
	body, _ := io.ReadAll(req.Body)
	content := fmt.Sprintf(fakeRequestTemplate, req.URL.RequestURI(), req.Host, DefaultUserAgent, len(body), string(body))
	log.Println(content)
	conn.Write([]byte(content))
	return http.ReadResponse(bufio.NewReader(conn), req)
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
	log.Println("json request is", body)
	encodedBody, _ := p.encrypt(body, iv) // encrypt our request
	log.Println("encrypted", encodedBody)
	formData := url.Values{"value": {encodedBody}}
	log.Println("post body", formData.Encode())
	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader([]byte(formData.Encode())))

	resp, err := fakeChromeRequest(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	// DO not parse invalid response, parse json only
	// if resp.ContentLength > 10*1024*1024 || !strings.Contains(resp.Header.Get("Content-Type"), "json") {
	// 	return nil, errors.New("invalid response")
	// }
	content, _ := io.ReadAll(resp.Body)
	// content := []byte(`{"Data":"61BzIrtQETynvjX3OTfc5pbRHnRBtOkYB5VSr28h8pc+CRH3GUdJfzkV6Z7HdGhoKDcwEIpBCtDkUotR9gGjUsxUnX4Tj6REmrNLtQ5B7TwmjScCdy1g+VhMKM83RWr1CHxbH+fBXb2PNMe/bVt+G3xpkUVcZDgSIJvuRPi6QurCf5hcOKg9TxaExA+p9CG3KoHDo9SJzpuma0+dBHy/fbMCu26D8xOlWCVTePKV4+krT04+l7XKrbl8t/fAiubnZ/OwaVVa0hSQOiqIJgqER2YONVPZbjgospUt/oiQ/xle+GVv1p1PZKUmVgcEKU9+BIlskdhQ+lmvU3CAf4t9XMFlDFoDJg3tXbO6WHldC9j1i8/kIR31bfxvZSin+wex+f1I0DjP8zC48lx1oHf95r8nfA5ImCNPrCO3xAI3OUxMzCtxBuB0sPxgNrjvDgo+4RGDaOppV9GMoozJTslBbuhawWJQf7pyW0Z/M609oFloNkT67poZY/C7bFI8XBPuYPDZNfSAwERi4j8C9gfsTjRl1/gbFFKIh9CGscpqzszKUjWppGrVMGtZoQBWG905IkWpJrfDa5D1XxhbXucN5q9WVn/MQO6Rd3dLApDbOnkNSTwfgR1NdukSrgJoX95gyY8yUgzVmYLkNbiBwVenTq9J9rG1xqbyktbrWqtVYRta+QaM7s9x3mNjTYaGwcDQRDeDuBAlsrOZAs15OxArwuP0cF09CgI+3eWqsnILdL4XHYNYaAUQUhwvWlPo+9Zh+saK4Ku6s+bD7vrSS+W3DLm06T8PUNs31RrsWOkzXuL+8q9IYkAWOUxtbCwV2oX9zzU0GjwHqkIsxcHO1yluYdaT/Xym0ra97zNy5O93v6+gAv2Aft/HLGlMpELRGDBkvofCA/frgDPY8/B7/K1ep1fq8Q2sy3Tq3ZUAPwYBUpny7H8JZR9SyXXUVMweLqRlP4Ss/mEyQZ9M1ep5F5Q98w==","Success":true,"Status":200,"ErrMessage":"成功"}`)
	log.Println("response:", string(content))
	var sgtvResp SGTVResponse
	json.Unmarshal(content, &sgtvResp)
	if sgtvResp.Success {
		cleartext, err := p.decrypt(sgtvResp.Data, iv)
		if err == nil {
			var chInfo SGTVChannelInfo
			json.Unmarshal(cleartext, &chInfo)
			log.Println(chInfo.Urls)
		}
	}
	return nil, NoMatchFeed
}

func init() {
	registerPlugin("4GTV", &SGTVParser{})
}
