package scraping

import "github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/session"

// URL is a result structure returned by a source
type URL struct {
	Source string
	Value  string
}

// Source is an interface inherited by each passive source
type Source interface {
	// domain - client - subs : URLS
	// Run takes a domain as argument and a session object
	// which contains the extractor for subdomains, http client
	// and other stuff.
	Run(string, *session.Session, bool) chan URL
	// Name returns the name of the source
	Name() string
}

// SourcesList contains the list of all sources. These sources are used by default.
var SourcesList = []string{
	"commoncrawl",
	"github",
	"otx",
	"urlscan",
	"wayback",
	"waybackrobots",
}
