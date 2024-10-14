package sources

// Source is an interface that defines methods for a data source.
// Each source is expected to implement a way to run an operation based on configuration and a domain,
// and provide its name.
type Source interface {
	// Run starts the data collection or scanning process for a specific domain.
	// It takes in a Configuration and a domain string as input and returns a channel
	// that emits Result structs. The channel is used for sending results back asynchronously.
	Run(config *Configuration, domain string) <-chan Result

	// Name returns the name of the source. This can be used to identify the data source
	// implementing the interface.
	Name() string
}

// Constants representing the names of different data sources.
// These sources could be APIs or services that are used to gather information about domains.
const (
	BEVIGIL            = "bevigil"     // Bevigil is an OSINT (Open-Source Intelligence) source.
	COMMONCRAWL        = "commoncrawl" // Common Crawl is a source of web data, commonly used in domain searches.
	GITHUB             = "github"      // GitHub source for finding code repositories and related metadata.
	INTELLIGENCEX      = "intelx"      // Intelligence X, a search engine and data archive.
	OPENTHREATEXCHANGE = "otx"         // Open Threat Exchange, a collaborative platform for sharing threat intelligence.
	URLSCAN            = "urlscan"     // URLScan.io, a service for scanning websites and collecting URLs.
	WAYBACK            = "wayback"     // Wayback Machine, an internet archive to retrieve historical versions of websites.
)

// List contains a collection of all available source names.
// This is useful for iterating over or referencing the supported data sources.
var List = []string{
	BEVIGIL,
	COMMONCRAWL,
	GITHUB,
	INTELLIGENCEX,
	OPENTHREATEXCHANGE,
	URLSCAN,
	WAYBACK,
}
