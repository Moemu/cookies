package contract

const (
	ErrorUnauthenticated           = "UNAUTHENTICATED"
	ErrorOrganizationAccessDenied  = "ORGANIZATION_ACCESS_DENIED"
	ErrorProjectAccessDenied       = "PROJECT_ACCESS_DENIED"
	ErrorProjectNotActive          = "PROJECT_NOT_ACTIVE"
	ErrorScopeRequired             = "SCOPE_REQUIRED"
	ErrorIdempotencyConflict       = "IDEMPOTENCY_CONFLICT"
	ErrorProviderJobNotReady       = "PROVIDER_JOB_NOT_READY"
	ErrorProviderOutputExpired     = "PROVIDER_OUTPUT_EXPIRED"
	ErrorProviderOutputUnavailable = "PROVIDER_OUTPUT_UNAVAILABLE"
	ErrorOutputMetadataMismatch    = "OUTPUT_METADATA_MISMATCH"
	ErrorAssetChecksumMismatch     = "ASSET_CHECKSUM_MISMATCH"
	ErrorAssetQuarantined          = "ASSET_QUARANTINED"
	ErrorAssetIntakeFailed         = "ASSET_INTAKE_FAILED"
	ErrorAssetNotReady             = "ASSET_NOT_READY"
)

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
