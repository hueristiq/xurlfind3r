package sources

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
