package sources

type Source interface {
	// Run takes in configuration which includes keys/tokens and other stuff,
	// and domain as arguments.
	Run(config *Configuration, domain string) <-chan Result
	// Name returns the name of the source.
	Name() string
}

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

// Result is a result structure returned by a source.
type Result struct {
	Type   ResultType
	Source string
	Value  string
	Error  error
}

// ResultType is the type of result returned by the source.
type ResultType int

// Types of results returned by the source.
const (
	URL ResultType = iota
	Error
)

var List = []string{
	"bevigil",
	"commoncrawl",
	"github",
	"intelx",
	"otx",
	"urlscan",
	"wayback",
}
