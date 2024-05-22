// ysp
package plugin

import (
	"bytes"
	"crypto/cipher"
	"crypto/des"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/zjyl1994/livetv/model"
)

type YSPParser struct {
}

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

type Kvcollect struct {
	BossId      string `json:"BossId"`
	Pwd         int32  `json:"Pwd"`
	Prog        string `json:"prog"`
	Playno      string `json:"playno"`
	Guid        string `json:"guid"`
	HhUa        string `json:"hh_ua"`
	Cdn         string `json:"cdn"`
	Sdtfrom     string `json:"sdtfrom"`
	Prd         int32  `json:"prd"`
	Platform    string `json:"platform"`
	Errcode     string `json:"errcode"`
	Durl        string `json:"durl"`
	Firstreport int32  `json:"firstreport"`
	SUrl        string `json:"sUrl"`
	SRef        string `json:"sRef"`
	Fplayerver  string `json:"fplayerver"`
	Livepid     string `json:"livepid"`
	Viewid      string `json:"viewid"`
	Seq         int64  `json:"seq"`
	Cmd         int32  `json:"cmd"`
	Geturltime  int64  `json:"geturltime"`
	Downspeed   int32  `json:"downspeed"`
	Defn        string `json:"defn"`
	Fact1       string `json:"fact1"`
	DC          string `json:"_dc"` // Private field based on naming convention
	LiveType    string `json:"live_type"`
	Ftime       string `json:"ftime"`
	Url         string `json:"url"`
	RandStr     string `json:"rand_str"`
	Signature   string `json:"signature"`
}

type Kvcollect2 struct {
	BossId      string `json:"BossId"`
	Pwd         int32  `json:"Pwd"`
	Prog        string `json:"prog"`
	Playno      string `json:"playno"`
	Guid        string `json:"guid"`
	HhUa        string `json:"hh_ua"`
	Cdn         string `json:"cdn"`
	Sdtfrom     string `json:"sdtfrom"`
	Prd         int32  `json:"prd"`
	Platform    string `json:"platform"`
	Errcode     string `json:"errcode"`
	Durl        string `json:"durl"`
	Firstreport int32  `json:"firstreport"`
	SUrl        string `json:"sUrl"`
	SRef        string `json:"sRef"`
	Fplayerver  string `json:"fplayerver"`
	Livepid     string `json:"livepid"`
	Viewid      string `json:"viewid"`
	Seq         string `json:"seq"`
	Logintype   string `json:"login_type"`
	HcOpenid    string `json:"hc_openid"`
	OpenId      string `json:"open_id"`
	Openid      string `json:"openid"`
	Cmd         int32  `json:"cmd"`
	Geturltime  int64  `json:"geturltime"`
	Downspeed   int32  `json:"downspeed"`
	Defn        string `json:"defn"`
	Fact1       string `json:"fact1"`
	DC          string `json:"_dc"` // Private field based on naming convention
	LiveType    string `json:"live_type"`
	Ftime       string `json:"ftime"`
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

func (p *YSPParser) Parse_unused(liveUrl string, lastInfo string) (*model.LiveInfo, error) {
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

const letterRunes = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

func getRand() string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, 10)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
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

func (p *YSPParser) Transform_unused(playList string, lastInfo string) (string, error) {
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

type PlayerInfo struct {
	ChannelId int64  `json:"channelId"`
	Livepid   int64  `json:"livepid"`
	Guid      string `json:"guid"`
	Lastbeat  int64  `json:"lastbeat"`
	Seq       int64  `json:"seq"`
}

func getSortedValues(data map[string]any) string {
	// Extract keys and sort them case-insensitively
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})

	// Build the formatted string
	var result string
	for i, key := range keys {
		if key == "signature" {
			continue
		}
		value := data[key]
		switch v := value.(type) {
		case string:
			result += fmt.Sprintf("%s=%s", key, v)
		case float32:
			result += fmt.Sprintf("%s=%.0f", key, v)
		case float64:
			result += fmt.Sprintf("%s=%.0f", key, v)
		case int32:
			result += fmt.Sprintf("%s=%d", key, v)
		case int64:
			result += fmt.Sprintf("%s=%d", key, v)
		}
		if i < len(keys)-1 {
			result += "&"
		}
	}
	return result
}

func makeSign(obj any, salt string) string {
	js, _ := json.Marshal(obj)
	var data map[string]any
	json.Unmarshal(js, &data)
	str2sign := getSortedValues(data)

	log.Println("string to sign", str2sign)
	h := md5.New()
	h.Write([]byte(str2sign + salt))
	return fmt.Sprintf("%x", h.Sum(nil))
}

//const TPL_SIGN_KVCOLLECT string = "BossId=%s&Pwd=%d&_dc=%s&cdn=%s&cmd=%d&defn=%s&downspeed=%d&durl=%s&errcode=%s&fact1=%s&firstreport=%d&fplayerver=%s&ftime=%s&geturltime=%d&guid=%s&hc_openid=&hh_ua=%s&live_type=&livepid=%s&login_type=&open_id=&openid=&platform=%s&playno=%s&prd=%d&prog=%s&rand_str=%s&sRef=%s&sUrl=%s&sdtfrom=%s&seq=%d&url=%s&viewid=%s"

const salt_kv string = "7s07jbb2Bwy31iPD"

func buildQuery(params map[string]any) string {
	query := url.Values{}
	svalue := ""
	for key, value := range params {
		switch v := value.(type) {
		case float32:
			svalue = fmt.Sprintf("%.0f", v)
		case float64:
			svalue = fmt.Sprintf("%.0f", v)
		case int32:
			svalue = fmt.Sprintf("%d", v)
		case int64:
			svalue = fmt.Sprintf("%d", v)
		case string:
			svalue = v
		}
		if svalue == "" {
			svalue = "-"
		}
		query.Add(key, svalue)
	}
	return query.Encode()
}

func (p *YSPParser) kvcollect1(liveinfo *model.LiveInfo) bool {
	var pInfo PlayerInfo
	var info Kvcollect
	now := time.Now()
	json.Unmarshal([]byte(liveinfo.ExtraInfo), &pInfo)

	info.BossId = "99150"
	info.Pwd = 1999332929
	info.Prd = 60000
	info.Playno = getRand()
	info.HhUa = DefaultUserAgent
	info.SUrl = "https://www.yangshipin.cn/tv/home"
	info.Cdn = "waibao"
	info.Prog = strconv.Itoa(int(pInfo.ChannelId))
	info.Guid = pInfo.Guid
	info.Livepid = strconv.Itoa(int(pInfo.Livepid))
	info.Viewid = strconv.Itoa(int(pInfo.ChannelId))
	info.Platform = "5910204"
	info.Defn = "fhd"
	info.Sdtfrom = "ysp_pc_01"
	info.Durl = liveinfo.LiveUrl
	info.Firstreport = 0
	info.Fplayerver = "180"
	info.Seq = pInfo.Seq
	info.Cmd = 263
	info.Geturltime = 0 // todo
	info.Downspeed = 10
	info.Fact1 = "ysp_pc_live_b"
	info.Ftime = now.Format("2006-01-02 15:04:05")
	info.Url = liveinfo.LiveUrl
	info.RandStr = getRand()

	if pInfo.Lastbeat == 0 {
		info.Firstreport = 1
	}

	info.Signature = makeSign(info, salt_kv)

	// now send the request
	client := http.Client{
		Timeout: time.Second * 10,
	}
	body := &bytes.Buffer{}
	writer := json.NewEncoder(body)
	writer.Encode(info)
	req, _ := http.NewRequest(http.MethodPost, "https://aatc-api.yangshipin.cn/kvcollect", body)
	req.Header.Set("Content-Type", "application/json")
	if resp, err := client.Do(req); err == nil {
		defer resp.Body.Close()
		result, _ := ioutil.ReadAll(resp.Body)
		//log.Println("kvcollect reply:", string(result))
		//log.Println("kvcollect succeeded")
		return strings.Contains(string(result), "success")
	} else {
		log.Println("kvcollect failed", err)
		return false
	}
}

func (p *YSPParser) kvcollect2(liveinfo *model.LiveInfo) bool {
	var pInfo PlayerInfo
	var info Kvcollect2
	now := time.Now()
	json.Unmarshal([]byte(liveinfo.ExtraInfo), &pInfo)

	info.BossId = "9150"
	info.Pwd = 1999332929
	info.Prd = 60000
	info.Playno = getRand()
	info.HhUa = DefaultUserAgent
	info.SUrl = "https://www.yangshipin.cn/tv/home"
	info.Cdn = "waibao"
	info.Prog = strconv.Itoa(int(pInfo.ChannelId))
	info.Guid = pInfo.Guid
	info.Livepid = strconv.Itoa(int(pInfo.Livepid))
	info.Viewid = strconv.Itoa(int(pInfo.ChannelId))
	info.Platform = "5910204"
	info.Defn = "fhd"
	info.Sdtfrom = "ysp_pc_01"
	info.Durl = liveinfo.LiveUrl
	info.Firstreport = 0
	info.Fplayerver = "180"
	info.Cmd = 263
	info.Geturltime = 0 // todo
	info.Downspeed = 10
	info.Fact1 = "ysp_pc_live_b"
	info.Ftime = now.Format("2006-01-02 15:04:05")

	if pInfo.Lastbeat == 0 {
		info.Firstreport = 1
	}

	data, _ := json.Marshal(info)
	var dataMap map[string]any
	json.Unmarshal(data, &dataMap)
	queryString := buildQuery(dataMap)
	log.Println("query string", queryString)
	body := bytes.NewBufferString(queryString)
	// now send the request
	client := http.Client{
		Timeout: time.Second * 10,
	}
	req, _ := http.NewRequest(http.MethodPost, "https://dtrace.ysp.cctv.cn/kvcollect", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if resp, err := client.Do(req); err == nil {
		defer resp.Body.Close()
		result, _ := ioutil.ReadAll(resp.Body)
		//log.Println("kvcollect2 reply:", string(result))
		//log.Println("kvcollect2 succeeded")
		return true
	} else {
		log.Println("kvcollect2 failed", err)
		return false
	}
}

func (p *YSPParser) Heartbeat(liveinfo *model.LiveInfo) {
	var pInfo PlayerInfo
	now := time.Now()
	json.Unmarshal([]byte(liveinfo.ExtraInfo), &pInfo)
	if time.Now().Unix()-pInfo.Lastbeat < 60 {
		return // no need to beat
	}

	result := p.kvcollect1(liveinfo) && p.kvcollect2(liveinfo)
	if result {
		pInfo.Seq++
		pInfo.Lastbeat = now.Unix()
		sJson, _ := json.Marshal(pInfo)
		liveinfo.ExtraInfo = string(sJson)
		//log.Println("kvcollect succeeded")
	} else {
		//log.Println("kvcollect failed")
	}
}

// check for playlist expiring.
// ysp playurl only stays valid for 5min, after that, a threshold will be applied
func (p *YSPParser) Check(content string, info *model.LiveInfo) error {
	var pInfo PlayerInfo
	var lastBeat int = 0
	json.Unmarshal([]byte(info.ExtraInfo), &pInfo)
	lastBeat = int(pInfo.Lastbeat) // in case heartbeated, we use the last beat time as srvtime
	if lastBeat == 0 {
		// if not, we use srvtime from url directly
		reg := regexp.MustCompile(`svrtime=(\d+)`)
		if matches := reg.FindStringSubmatch(info.LiveUrl); matches != nil {
			lastBeat, _ = strconv.Atoi(matches[1])
		}
	}

	// a url expires after 2mins
	if time.Now().Unix()-int64(lastBeat) > 120 {
		log.Println("ysp playlist expired")
		return errors.New("expired")
	} else {
		p.Heartbeat(info)
	}
	return nil
}

func (p *YSPParser) Parse(liveUrl string, proxyUrl string, previousExtraInfo string) (*model.LiveInfo, error) {
	client := http.Client{
		Timeout:   time.Second * 10,
		Transport: transportWithProxy(proxyUrl),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
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
	body, _ := ioutil.ReadAll(resp.Body)
	redir := resp.Header.Get("Location")
	if redir == "" {
		return nil, NoMatchFeed
	}
	directParser := &DirectM3U8Parser{}
	model, err := directParser.Parse(redir, proxyUrl, previousExtraInfo)
	if err == nil {
		model.ExtraInfo = string(body)
	}
	return model, err
}

func init() {
	registerPlugin("YSP", &YSPParser{})
}
