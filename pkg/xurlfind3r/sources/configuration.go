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
	GitHub  []string `yaml:"github"`
	Intelx  []string `yaml:"intelx"`
	URLScan []string `yaml:"urlscan"`
}
