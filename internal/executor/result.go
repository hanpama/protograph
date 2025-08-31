package executor

// GraphQLError represents an error that occurred during execution
type GraphQLError struct {
	Message    string         `json:"message"`
	Path       Path           `json:"path,omitempty"`
	Extensions map[string]any `json:"extensions,omitempty"`
}

func (e GraphQLError) Error() string {
	return e.Message
}

// ExecutionResult represents the result of executing a GraphQL query
type ExecutionResult struct {
	Data   any            `json:"data"`
	Errors []GraphQLError `json:"errors,omitempty"`
}
