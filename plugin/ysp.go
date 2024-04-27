// ysp
package plugin

import (
	"bytes"
	"crypto/cipher"
	"crypto/des"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/zjyl1994/livetv/model"
)

type YSPParser struct{}

type YSPResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Acode             int               `json:"acode"`
		BackurlList       []Backurl         `json:"backurl_list"`
		BulletFlag        int               `json:"bullet_flag"`
		CdnName           string            `json:"cdn_name"`
		Chanll            string            `json:"chanll"`
		Cnlid             int               `json:"cnlid"`
		ContentID         string            `json:"content_id"`
		Defn              string            `json:"defn"`
		Encrypt           int               `json:"encrypt"`
		EncryptInfo       string            `json:"encrypt_info"`
		Errinfo           string            `json:"errinfo"`
		ExtendedParam     string            `json:"extended_param"`
		Formats           []Format          `json:"formats"`
		HttpHeaders       map[string]string `json:"http_header"`
		Iretcode          int               `json:"iretcode"`
		Iretdetailcode    int               `json:"iretdetailcode"`
		IsQuic            int               `json:"is_quic"`
		LiveTrans         int               `json:"live_trans"`
		Livepid           int               `json:"livepid"`
		Message           string            `json:"message"`
		Playback          Playback          `json:"playback"`
		Playtime          int               `json:"playtime"`
		Playurl           string            `json:"playurl"`
		Previewcnt        int               `json:"previewcnt"`
		Restpreviewcnt    int               `json:"restpreviewcnt"`
		RtcStopUrl        string            `json:"rtc_stop_url"`
		RtcUrl            string            `json:"rtc_url"`
		Sig               string            `json:"sig"`
		Stream            int               `json:"stream"`
		Targetid          string            `json:"targetid"`
		Totalplaytime     int               `json:"totalplaytime"`
		Vcode             int               `json:"vcode"`
		Vkey              string            `json:"vkey"`
		VkeyRenewInterval int               `json:"vkey_renew_interval"`
		Watermark         int               `json:"watermark"`
	} `json:"data"`
}

type Backurl struct {
	Url string `json:"url"`
}

type Format struct {
	Defnname string `json:"defnname"`
	Defnrate string `json:"defnrate"`
	Encrypt  int    `json:"encrypt"`
	Fn       string `json:"fn"`
	Fnname   string `json:"fnname"`
	Id       int    `json:"id"`
}

type Playback struct {
	Playbacktime int `json:"playbacktime"`
	Svrtick      int `json:"svrtick"`
}

type DesInfo struct {
	Key           string
	IV            string
	ExtendedParam string
}

func extractDESKV(code string) (string, string) {
	patternKey := regexp.MustCompile(`var des_key = "(.*?)";`)
	patternIv := regexp.MustCompile(`var des_iv = "(.*?)";`)

	// Initialize variables to store extracted values
	desKey := ""
	desIv := ""

	// Use regex to extract the value of des_key
	if matchesKey := patternKey.FindStringSubmatch(code); matchesKey != nil {
		desKey = matchesKey[1]
	}

	// Use regex to extract the value of des_iv
	if matchesIv := patternIv.FindStringSubmatch(code); matchesIv != nil {
		desIv = matchesIv[1]
	}
	return desKey, desIv
}

func (p *YSPParser) Parse(liveUrl string, lastInfo string) (*model.LiveInfo, error) {
	client := http.Client{
		Timeout: time.Second * 10,
	}
	req, err := http.NewRequest("GET", liveUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", DefaultUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	// the body should contain a valid ysp response
	if resp.ContentLength > 10*1024*1024 || !strings.Contains(resp.Header.Get("Content-Type"), "json") {
		return nil, errors.New("invalid response")
	}
	content, _ := io.ReadAll(resp.Body)
	var yspresp YSPResponse
	err = json.Unmarshal(content, &yspresp)
	if err != nil {
		return nil, err
	}
	if yspresp.Code != 0 {
		return nil, NoMatchFeed
	}
	// probe dynamic des key and iv
	var data map[string]string
	err = json.Unmarshal([]byte(yspresp.Data.Chanll), &data)
	if err != nil {
		return nil, err
	}
	chanllCode, _ := base64.StdEncoding.DecodeString(data["code"])
	key, iv := extractDESKV(string(chanllCode))
	di := &DesInfo{
		Key:           key,
		IV:            iv,
		ExtendedParam: yspresp.Data.ExtendedParam,
	}
	info, _ := json.Marshal(di)

	li := &model.LiveInfo{
		LiveUrl:   yspresp.Data.Playurl,
		Logo:      "",
		ExtraInfo: string(info),
	}
	return li, nil
}

type EncryptData struct {
	Mver    string `json:"mver"`
	Subver  string `json:"subver"`
	Host    string `json:"host"`
	Referer string `json:"referer"`
	Canvas  string `json:"canvas"`
}

func pKCS5Padding(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}

func padKey(key []byte, desiredLength int) []byte {
	if len(key) >= desiredLength {
		return key[:desiredLength]
	}

	padding := make([]byte, desiredLength-len(key))
	return append(key, padding...)
}

// Encrypt data function
func encryptDESede3CBC(plainText []byte, key string, iv string) (string, error) {
	bKey, _ := base64.StdEncoding.DecodeString(key)
	bIV, _ := base64.StdEncoding.DecodeString(iv)
	block, err := des.NewTripleDESCipher(padKey(bKey, 24))
	if err != nil {
		fmt.Printf("init err: %s\r\n", err)
		return "", err
	}

	blockSize := block.BlockSize()
	paddedPlaintext := pKCS5Padding(plainText, blockSize)

	// Encrypt with CBC mode
	cipherText := make([]byte, len(paddedPlaintext))
	encryptMode := cipher.NewCBCEncrypter(block, bIV)
	encryptMode.CryptBlocks(cipherText, paddedPlaintext)

	return strings.ToUpper(hex.EncodeToString(cipherText)), nil
}

func (p *YSPParser) Transform(playList string, lastInfo string) (string, error) {
	var di DesInfo
	err := json.Unmarshal([]byte(lastInfo), &di)

	if err != nil {
		return playList, nil
	}

	metaData := EncryptData{
		Mver:    "1",
		Subver:  "1.2",
		Host:    "www.yangshipin.cn/#/tv/home?pid=",
		Referer: "",
		Canvas:  "YSPANGLE(Intel,Intel(R)Iris(R)XeGraphics(0x000046A6)Direct3D11vs_5_0ps_5_0,D3D11)",
	}
	metaBytes, _ := json.Marshal(metaData)
	encryptedHex, err := encryptDESede3CBC(metaBytes, di.Key, di.IV)
	if err != nil {
		return playList, nil
	}
	return playList + "&revoi=" + encryptedHex + di.ExtendedParam, nil
}

func init() {
	registerPlugin("YSP", &YSPParser{})
}
