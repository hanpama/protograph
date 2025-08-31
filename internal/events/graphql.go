package events

import "time"

// GraphQLStart is emitted before executing a GraphQL operation.
type GraphQLStart struct {
	Query         string
	OperationName string
	OperationType string
}

// GraphQLFinish is emitted after executing a GraphQL operation.
type GraphQLFinish struct {
	Query         string
	OperationName string
	OperationType string
	Errors        []error
	Duration      time.Duration
}
