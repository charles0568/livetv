package model

type Channel struct {
	ID     uint `gorm:"primary_key"`
	Name   string
	Logo   string
	URL    string
	Parser string
	Proxy  bool
	Token  string `gorm:"-:all"`
}

type LiveInfo struct {
	LiveUrl   string
	Logo      string
	ExtraInfo string
}
