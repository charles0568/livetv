package route

import (
	"github.com/gin-gonic/gin"
	"github.com/zjyl1994/livetv/handler"
)

func Register(r *gin.Engine) {
	r.GET("/lives.m3u", handler.M3UHandler)
	r.GET("/live.m3u8", handler.LiveHandler)
	r.HEAD("/live.m3u8", handler.LivePreHandler)
	r.GET("/live.ts", handler.TsProxyHandler)
	r.GET("/cache.txt", handler.CacheHandler)

	r.GET("/api/channels", handler.ChannelListHandler)
	r.GET("/api/plugins", handler.PluginListHandler)
	r.GET("/api/crsf", handler.CRSFHandler)
	r.POST("/api/newchannel", handler.NewChannelHandler)
	r.POST("/api/updatechannel", handler.UpdateChannelHandler)
	r.GET("/api/getconfig", handler.GetConfigHandler)
	r.GET("/api/delchannel", handler.DeleteChannelHandler)
	r.POST("/api/updconfig", handler.UpdateConfigHandler)
	r.GET("/api/auth", handler.AuthProbeHandler)
	r.GET("/log", handler.LogHandler)
	// r.GET("/login", handler.LoginViewHandler)
	r.POST("/api/login", handler.LoginActionHandler)
	r.GET("/api/logout", handler.LogoutHandler)
	r.POST("/api/changepwd", handler.ChangePasswordHandler)
	r.GET("/", handler.IndexHandler)
	r.GET("/:path", handler.IndexHandler)
}
