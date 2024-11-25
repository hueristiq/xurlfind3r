package sources

// Source is an interface that defines the blueprint for a data source.
// Any data source integrated into the application must implement this interface.
// It provides methods for initiating data collection and identifying the source.
//
// Methods:
//   - Run: Executes the data collection or scanning process for a specific domain
//     and configuration, returning results asynchronously through a channel.
//   - Name: Returns the name of the data source, which is helpful for logging,
//     debugging, or identifying the source in reports.
type Source interface {
	// Run starts the data collection process for the specified domain using the provided
	// configuration. It returns a channel that emits `Result` structs asynchronously,
	// allowing concurrent processing and streaming of results.
	//
	// Parameters:
	// - cfg *Configuration: The configuration settings, such as API keys and options,
	//   required to interact with the data source.
	// - domain string: The target domain for which data is to be collected.
	//
	// Returns:
	// - <-chan Result: A channel that streams the results produced by the data source.
	Run(config *Configuration, domain string) <-chan Result
	// Name retrieves the unique identifier or name of the data source. This is primarily
	// used for distinguishing between multiple sources, logging activity, and reporting
	// during subdomain enumeration.
	//
	// Returns:
	// - string: The name of the data source.
	Name() string
}

// The following constants define the names of supported data sources.
// These constants are used as unique identifiers for the corresponding sources.
// Each source provides different types of data, such as subdomains, SSL/TLS certificates,
// historical records, or vulnerability data.
//
// List of Constants:
//
// - BEVIGIL: Focuses on vulnerabilities in mobile applications.
// - COMMONCRAWL: Repository of open web data.
// - GITHUB: Searches code repositories for relevant data.
// - INTELLIGENCEX: Search engine for intelligence data.
// - OPENTHREATEXCHANGE: Collaborative threat intelligence platform.
// - URLSCAN: Service for website scanning and URL collection.
// - WAYBACK: Internet archive for historical website snapshots.
const (
	BEVIGIL            = "bevigil"
	COMMONCRAWL        = "commoncrawl"
	GITHUB             = "github"
	INTELLIGENCEX      = "intelx"
	OPENTHREATEXCHANGE = "otx"
	URLSCAN            = "urlscan"
	WAYBACK            = "wayback"
)

// List is a collection of all supported source names.
// It provides a convenient way to iterate over, validate, or configure the data sources dynamically.
// Developers can use this list for tasks such as enabling specific sources or verifying
// that a provided source name is valid.
//
// Usage:
// - Iterate over the List to dynamically load supported sources.
// - Validate user input by checking against the entries in the List.
//
// Example:
//
//	for _, source := range List {
//	    fmt.Println("Supported source:", source)
//	}
var List = []string{
	BEVIGIL,
	COMMONCRAWL,
	GITHUB,
	INTELLIGENCEX,
	OPENTHREATEXCHANGE,
	URLSCAN,
	WAYBACK,
}
