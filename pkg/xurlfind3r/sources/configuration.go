package sources

import (
	"regexp"
)

type Configuration struct {
	Domain             string
	IncludeSubdomains  bool
	Keys               Keys
	ParseWaybackRobots bool
	ParseWaybackSource bool
	URLsRegex          *regexp.Regexp
	MediaURLsRegex     *regexp.Regexp
	RobotsURLsRegex    *regexp.Regexp
}

type Keys struct {
	GitHub     []string `json:"github"`
	Intelx     []string `json:"intelx"` // unused, add for the purpose of adding an asterisk `*` on listing sources
	IntelXHost string   `json:"intelXHost"`
	IntelXKey  string   `json:"intelXKey"`
	URLScan    []string `json:"urlscan"`
}
