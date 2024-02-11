package handler

type Channel struct {
	ID         uint
	Name       string
	URL        string
	M3U8       string
	Proxy      bool
	LastUpdate string
	Status     int
	Message    string
}

type Config struct {
	BaseURL string `json:"baseurl"`
	Cmd     string `json:"cmd"`
	Args    string `json:"args"`
	ApiKey  string `json:"apikey"`
}
