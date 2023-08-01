package sources

type Configuration struct {
	IncludeSubdomains  bool
	Keys               Keys
	ParseWaybackRobots bool
	ParseWaybackSource bool
}

type Keys struct {
	Bevigil []string `yaml:"bevigil"`
	GitHub  []string `yaml:"github"`
	Intelx  []string `yaml:"intelx"`
	URLScan []string `yaml:"urlscan"`
}
