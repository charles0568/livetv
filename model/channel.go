package model

type Channel struct {
	ID    uint `gorm:"primary_key"`
	Name  string
	Logo  string
	URL   string
	Proxy bool
}
