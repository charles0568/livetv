package model

type Channel struct {
	ID       uint `gorm:"primary_key"`
	Name     string
	Logo     string
	URL      string
	Parser   string
	Proxy    bool
	ProxyUrl string // proxy for server connection
	Token    string `gorm:"-:all"`
}

type LiveInfo struct {
	LiveUrl   string
	Logo      string
	ExtraInfo string
}
