package sources

type Keys struct {
	GitHub     []string `json:"github"`
	Intelx     string   `json:"intelx"` // unused, add for the purpose of adding an asterisk `*` on listing sources
	IntelXHost string   `json:"intelXHost"`
	IntelXKey  string   `json:"intelXKey"`
}

type Configuration struct {
	Keys               Keys
	IncludeSubdomains  bool
	WaybackParseRobots bool
	WaybackParseSource bool
}
