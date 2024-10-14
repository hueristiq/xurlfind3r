package sources

// Result represents the outcome of an operation or request, including the type of result,
// the source of the data, the actual value retrieved (if applicable), and any error encountered.
type Result struct {
	Type   ResultType // Specifies the type of result (e.g., a URL or an error).
	Source string     // Indicates the source from which the result was obtained (e.g., a specific API or service).
	Value  string     // Holds the value of the result, such as a URL or any other data returned from the operation.
	Error  error      // Holds any error that occurred during the operation, or nil if no error occurred.
}

// ResultType defines the type of result using an integer type. It can represent different
// kinds of outcomes from an operation, such as a URL or an error.
type ResultType int

const (
	ResultURL   ResultType = iota // Represents a successful result containing a URL.
	ResultError                   // Represents a result where an error occurred during the operation.
)
