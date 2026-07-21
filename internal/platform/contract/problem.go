package contract

// Problem is the stable error envelope returned by HTTP handlers. Internal
// causes, tokens, SQL statements, and provider responses must never be placed
// in Message or Details.
type Problem struct {
	Error Error `json:"error"`
}

type Error struct {
	Code      string           `json:"code"`
	Message   string           `json:"message"`
	RequestID string           `json:"request_id"`
	Retryable bool             `json:"retryable"`
	Details   []FieldViolation `json:"details"`
	HelpURL   *string          `json:"help_url"`
}

type FieldViolation struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}
