package handler

import (
	"embed"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/zjyl1994/livetv/model"
	"github.com/zjyl1994/livetv/service"
	"github.com/zjyl1994/livetv/util"

	"golang.org/x/text/language"
)

var langMatcher = language.NewMatcher([]language.Tag{
	language.English,
	language.Chinese,
})

//go:embed web
var webFS embed.FS

func IndexHandler(c *gin.Context) {
	fullPath := strings.ReplaceAll(filepath.Join("web", c.Param("path")), "\\", "/")

	// Check if file exists
	f, err := webFS.Open(fullPath)
	if f != nil {
		fi, _ := f.Stat()
		if fi.IsDir() {
			err = errors.New("can't serve a folder")
			f.Close()
		}
	}
	if err != nil {
		// Not found, serve index.html
		fullPath = strings.ReplaceAll(filepath.Join("web", "index.html"), "\\", "/")
		f, err = webFS.Open(fullPath)
	}

	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	defer f.Close()
	c.Writer.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(fullPath)))
	io.Copy(c.Writer, f)
}

func loadConfig() (Config, error) {
	var conf Config
	if cmd, err := service.GetConfig("ytdl_cmd"); err != nil {
		return conf, err
	} else {
		conf.Cmd = cmd
	}
	if args, err := service.GetConfig("ytdl_args"); err != nil {
		return conf, err
	} else {
		conf.Args = args
	}
	if burl, err := service.GetConfig("base_url"); err != nil {
		return conf, err
	} else {
		conf.BaseURL = burl
	}
	return conf, nil
}

func CRSFHandler(c *gin.Context) {
	session := sessions.Default(c)
	// session.Options(sessions.Options{
	// 	SameSite: http.SameSiteNoneMode,
	// 	Secure:   true,
	// })
	crsfToken := util.RandString(10)
	session.Set("crsfToken", crsfToken)
	err := session.Save()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	} else {
		c.Data(http.StatusOK, "text/plain", []byte(crsfToken))
	}
}

func ChannelListHandler(c *gin.Context) {
	if sessions.Default(c).Get("logined") != true {
		c.String(http.StatusUnauthorized, "Unauthorized")
		return
	}
	baseUrl, err := service.GetConfig("base_url")
	if err != nil {
		log.Println(err.Error())
		c.String(http.StatusInternalServerError, "error: %s", err.Error())
		return
	}
	channelModels, err := service.GetAllChannel()
	if err != nil {
		log.Println(err.Error())
		c.String(http.StatusInternalServerError, "error: %s", err.Error())
		return
	}
	channels := make([]Channel, len(channelModels)+1)
	channels[0] = Channel{
		ID:   0,
		Name: "playlist",
		M3U8: baseUrl + "/lives.m3u",
	}
	for i, v := range channelModels {
		status := service.GetStatus(v.URL)
		channels[i+1] = Channel{
			ID:         v.ID,
			Name:       v.Name,
			URL:        v.URL,
			M3U8:       baseUrl + "/live.m3u8?c=" + strconv.Itoa(int(v.ID)),
			Proxy:      v.Proxy,
			LastUpdate: status.Time.Format("2006-01-02 15:04:05"),
			Status:     status.Status,
			Message:    status.Msg,
		}
	}
	c.JSON(http.StatusOK, channels)
}

func NewChannelHandler(c *gin.Context) {
	if sessions.Default(c).Get("logined") != true {
		c.String(http.StatusUnauthorized, "Unauthorized")
		return
	}
	chName := c.PostForm("name")
	chURL := c.PostForm("url")
	if chName == "" || chURL == "" {
		c.String(http.StatusBadRequest, "Incomplete channel info")
		return
	}
	chProxy := c.PostForm("proxy") != ""
	mch := model.Channel{
		Name:  chName,
		URL:   chURL,
		Proxy: chProxy,
	}
	err := service.SaveChannel(mch)
	if err != nil {
		log.Println(err.Error())
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.String(http.StatusOK, "")
	go service.UpdateURLCacheSingle(chURL) // update liveURL on adding new channel
}

func AuthProbeHandler(c *gin.Context) {
	if sessions.Default(c).Get("logined") != true {
		c.String(http.StatusUnauthorized, "Unauthorized")
	} else {
		c.String(http.StatusOK, "")
	}
}

func UpdateChannelHandler(c *gin.Context) {
	if sessions.Default(c).Get("logined") != true {
		c.String(http.StatusUnauthorized, "Unauthorized")
		return
	}
	chID := util.String2Uint(c.PostForm("id"))
	if chID == 0 {
		c.String(http.StatusInternalServerError, "empty id")
		return
	}
	chName := c.PostForm("name")
	chURL := c.PostForm("url")
	if chName == "" || chURL == "" {
		c.String(http.StatusBadRequest, "Incomplete channel info")
		return
	}
	chProxy := c.PostForm("proxy") == "true"
	mch := model.Channel{
		ID:    chID,
		Name:  chName,
		URL:   chURL,
		Proxy: chProxy,
	}
	err := service.SaveChannel(mch)
	if err != nil {
		log.Println(err.Error())
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.String(http.StatusOK, "")
	go service.UpdateURLCacheSingle(chURL) // update liveURL on updating new channel
}

func DeleteChannelHandler(c *gin.Context) {
	if sessions.Default(c).Get("logined") != true {
		c.String(http.StatusUnauthorized, "Unauthorized")
		return
	}
	chID := util.String2Uint(c.Query("id"))
	if chID == 0 {
		c.String(http.StatusInternalServerError, "empty id")
		return
	}
	err := service.DeleteChannel(chID)
	if err != nil {
		log.Println(err.Error())
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.String(http.StatusOK, "")
}

func GetConfigHandler(c *gin.Context) {
	if sessions.Default(c).Get("logined") != true {
		c.String(http.StatusUnauthorized, "Unauthorized")
		return
	}
	conf, err := loadConfig()
	if err != nil {
		log.Println(err.Error())
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, conf)
}

func UpdateConfigHandler(c *gin.Context) {
	if sessions.Default(c).Get("logined") != true {
		c.String(http.StatusUnauthorized, "Unauthorized")
		return
	}
	ytdlCmd := c.PostForm("cmd")
	ytdlArgs := c.PostForm("args")
	baseUrl := strings.TrimSuffix(c.PostForm("baseurl"), "/")
	if len(ytdlCmd) > 0 {
		err := service.SetConfig("ytdl_cmd", ytdlCmd)
		if err != nil {
			log.Println(err.Error())
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
	}
	if len(ytdlArgs) > 0 {
		err := service.SetConfig("ytdl_args", ytdlArgs)
		if err != nil {
			log.Println(err.Error())
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
	}
	if len(baseUrl) > 0 {
		err := service.SetConfig("base_url", baseUrl)
		if err != nil {
			log.Println(err.Error())
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
	}
	c.String(http.StatusOK, "")
}

func LogHandler(c *gin.Context) {
	if sessions.Default(c).Get("logined") != true {
		c.String(http.StatusUnauthorized, "Unauthorized")
		return
	}
	c.File(os.Getenv("LIVETV_DATADIR") + "/livetv.log")
}

func LoginViewHandler(c *gin.Context) {
	session := sessions.Default(c)
	crsfToken := util.RandString(10)
	session.Set("crsfToken", crsfToken)
	err := session.Save()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"ErrMsg": err.Error(),
		})
		return
	}
	c.HTML(http.StatusOK, "login.html", gin.H{
		"Crsf": crsfToken,
	})
}

func LoginActionHandler(c *gin.Context) {
	session := sessions.Default(c)
	// session.Options(sessions.Options{
	// 	SameSite: http.SameSiteNoneMode,
	// 	Secure:   true,
	// })
	crsfToken := c.PostForm("crsf")
	if crsfToken != session.Get("crsfToken") {
		log.Println(crsfToken, session.Get("crsfToken"))
		c.String(http.StatusBadRequest, "bad request")
		return
	}
	pass := c.PostForm("password")
	cfgPass, err := service.GetConfig("password")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	if pass == cfgPass {
		session.Set("logined", true)
		err = session.Save()
		if err != nil {
			log.Println(err.Error())
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.String(http.StatusOK, "ok")
	} else {
		c.String(http.StatusForbidden, "Password error!")
	}
}

func LogoutHandler(c *gin.Context) {
	if sessions.Default(c).Get("logined") != true {
		c.String(http.StatusOK, "")
		return
	}
	session := sessions.Default(c)
	session.Delete("logined")
	err := session.Save()
	if err != nil {
		log.Println(err.Error())
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.String(http.StatusOK, "")
}

func ChangePasswordHandler(c *gin.Context) {
	if sessions.Default(c).Get("logined") != true {
		c.String(http.StatusUnauthorized, "Unauthorized")
		return
	}
	pass := c.PostForm("password")
	pass2 := c.PostForm("password2")
	if pass == "" {
		c.String(http.StatusBadRequest, "Empty password!")
	}
	if pass != pass2 {
		c.String(http.StatusBadRequest, "Password mismatch!")
	}
	err := service.SetConfig("password", pass)
	if err != nil {
		log.Println(err.Error())
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	LogoutHandler(c)
}
