package main

import (
	"bytes"
	"context"
	"crypto/cipher"
	"crypto/des"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"
	"github.com/robfig/cron/v3"
	"github.com/zjyl1994/livetv/global"
	"github.com/zjyl1994/livetv/plugin"
	"github.com/zjyl1994/livetv/route"
	"github.com/zjyl1994/livetv/service"
)

func demo() {
	p, _ := plugin.GetPlugin("4GTV")
	li, err := p.Parse("https://4gtv.tv/channel/4gtv-4gtv002?set=4&ch=3", "")
	if err != nil {
		log.Println(err)
	} else {
		log.Println("liveurl:", li.LiveUrl)
	}
}

type EncryptData struct {
	Mver    string `json:"mver"`
	Subver  string `json:"subver"`
	Host    string `json:"host"`
	Referer string `json:"referer"`
	Canvas  string `json:"canvas"`
}

func PKCS5Padding(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}

func doDes() {
	padKey := func(key []byte, desiredLength int) []byte {
		if len(key) >= desiredLength {
			return key[:desiredLength]
		}

		padding := make([]byte, desiredLength-len(key))
		return append(key, padding...)
	}
	// Encrypt data function
	encryptDESede3CBC := func(plainText []byte, key string, iv string) (string, error) {
		bKey, _ := base64.StdEncoding.DecodeString(key)
		bIV, _ := base64.StdEncoding.DecodeString(iv)
		block, err := des.NewTripleDESCipher(padKey(bKey, 24))
		if err != nil {
			fmt.Printf("init err: %s\r\n", err)
			return "", err
		}

		blockSize := block.BlockSize()
		paddedPlaintext := PKCS5Padding(plainText, blockSize)

		// Encrypt with CBC mode
		cipherText := make([]byte, len(paddedPlaintext))
		encryptMode := cipher.NewCBCEncrypter(block, bIV)
		encryptMode.CryptBlocks(cipherText, paddedPlaintext)

		return strings.ToUpper(hex.EncodeToString(cipherText)), nil
	}

	metaData := EncryptData{
		Mver:    "1",
		Subver:  "1.2",
		Host:    "www.yangshipin.cn/#/tv/home?pid=",
		Referer: "",
		Canvas:  "YSPANGLE(Intel,Intel(R)Iris(R)XeGraphics(0x000046A6)Direct3D11vs_5_0ps_5_0,D3D11)",
	}
	metaBytes, _ := json.Marshal(metaData)
	encryptedHex, err := encryptDESede3CBC(metaBytes, "eCfOXZ/4/f9NEgM2MFIXSQ==", "Ylxh0nCZ9r0=")
	if err != nil {
		return
	}
	fmt.Println(encryptedHex)

}

func main() {
	pwd := flag.String("pwd", "", "reset password")
	listen := flag.String("listen", ":9000", "listening address")
	disableProtection := flag.Bool("disable-protection", false, "temporarily disable token protection")
	flag.Parse()
	datadir := os.Getenv("LIVETV_DATADIR")
	if datadir == "" {
		ex, err := os.Executable()
		if err != nil {
			panic(err)
		}
		datadir = filepath.Join(filepath.Dir(ex), "data")
		os.Setenv("LIVETV_DATADIR", datadir)
	}
	os.Mkdir(datadir, os.ModePerm)

	if *pwd != "" {
		// reset password
		err := global.InitDB(datadir + "/livetv.db")
		if err != nil {
			log.Panicf("init: %s\n", err)
		}
		err = global.SetConfig("password", *pwd)
		if err == nil {
			log.Println("Password has been changed.")
		} else {
			log.Println("Failed to reset password:", err.Error())
		}
		return
	}

	if *disableProtection {
		os.Setenv("LIVETV_FREEACCESS", "1")
	}

	binding := os.Getenv("LIVETV_LISTEN")
	if binding == "" {
		binding = *listen
	}
	rand.Seed(time.Now().UnixNano())
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Server listen", binding)
	log.Println("Server datadir", datadir)
	logFile, err := os.OpenFile(datadir+"/livetv.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Panicln(err)
	}
	log.SetOutput(io.MultiWriter(os.Stderr, logFile))
	err = global.InitDB(datadir + "/livetv.db")
	if err != nil {
		log.Panicf("init: %s\n", err)
	}
	log.Println("LiveTV starting...")
	go service.LoadChannelCache()
	c := cron.New()
	_, err = c.AddFunc("0 */3 * * *", service.UpdateURLCache)
	if err != nil {
		log.Panicf("preloadCron: %s\n", err)
	}
	c.Start()
	sessionSecert, err := global.GetConfig("password")
	if err != nil {
		sessionSecert = "sessionSecert"
	}
	// ignore tls cert error
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	store := cookie.NewStore([]byte(sessionSecert))
	/* CORS */
	/*config := cors.DefaultConfig()
	config.AllowOrigins = []string{"http://localhost:8000"}
	config.AllowCredentials = true
	router.Use(cors.New(config))*/
	router.Use(sessions.Sessions("mysession", store))
	// router.Static("/", "./web")
	route.Register(router)
	srv := &http.Server{
		Addr:    binding,
		Handler: router,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Panicf("listen: %s\n", err)
		}
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shuting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Panicf("Server forced to shutdown: %s\n", err)
	}
	log.Println("Server exiting")
	logFile.Close()
}
