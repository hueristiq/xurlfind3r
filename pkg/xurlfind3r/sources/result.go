package sources

// Result represents the outcome of URL discovery. It encapsulates details about the result, including
// its type, the originating source, the actual data (if available), and any error
// encountered during the operation.
//
// Fields:
//   - Type (ResultType): Specifies the kind of result (e.g., URL or error).
//   - Source (string): Identifies the source that produced this result (e.g., "bevigil", "github").
//   - Value (string): Contains the actual URL retrieved from the source.
//     This field is empty if the result is an error.
//   - Error (error): Holds the error encountered during the operation, if any. If no error
//     occurred, this field is nil.
//
// Usage:
//   - Results are generated by sources implementing the `Source` interface. They are used
//     to communicate the findings or errors encountered during the operation.
//   - A `Result` of type `ResultError` indicates an issue with the operation, and the
//     `Error` field provides more context about the failure.
//
// Example:
//
//	result := Result{
//	    Type:   ResultURL,
//	    Source: "bevigil",
//	    Value:  "https://example.com",
//	    Error:  nil,
//	}
type Result struct {
	Type   ResultType
	Source string
	Value  string
	Error  error
}

// ResultType defines the category of a `Result` using an integer enumeration.
// It allows for distinguishing between different types of outcomes produced by sources.
//
// Enumeration Values:
//   - ResultURL: Indicates a successful result containing a URL retrieved from the source.
//   - ResultError: Represents a result indicating that an error occurred during the operation.
//
// Usage:
//   - Use `ResultType` to differentiate between successful and erroneous outcomes when
//     processing results returned by sources.
//
// Example:
//
//	switch result.Type {
//	case ResultURL:
//	    fmt.Printf("URL found: %s\n", result.Value)
//	case ResultError:
//	    fmt.Printf("Error occurred: %v\n", result.Error)
//	}
type ResultType int

// Constants representing the types of results that can be produced by a source.
//
// List of Constants:
//   - ResultURL: Represents a successful result containing subdomain.
//   - ResultError: Indicates an error encountered during the operation, with details
//     provided in the `Error` field of the `Result`.
//
// Example:
//
//	result := Result{
//	    Type:   ResultURL,
//	    Source: "bevigil",
//	    Value:  "https://example.com",
//	    Error:  nil,
//	}
const (
	ResultURL ResultType = iota
	ResultError
)
