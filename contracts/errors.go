package contracts

import "fmt"

const (
	ErrorInvalidInput        = "INVALID_INPUT"
	ErrorNotFound            = "NOT_FOUND"
	ErrorStaleVersion        = "STALE_VERSION"
	ErrorEventConflict       = "EVENT_CONFLICT"
	ErrorIdempotencyConflict = "IDEMPOTENCY_CONFLICT"
	ErrorGeoUnavailable      = "GEO_UNAVAILABLE"
	ErrorInvalidPlan         = "INVALID_PLAN"
	ErrorComputationFailed   = "COMPUTATION_FAILED"
)

type ContractError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
	cause   error
}

var _ error = (*ContractError)(nil)

func (e *ContractError) Error() string {
	return e.Code + ": " + e.Message
}

func (e *ContractError) Unwrap() error {
	return e.cause
}

func NewError(code, message string, details map[string]any) *ContractError {
	if details == nil {
		details = map[string]any{}
	}
	return &ContractError{Code: code, Message: message, Details: details}
}

func WrapError(code, message string, cause error, details map[string]any) *ContractError {
	err := NewError(code, message, details)
	err.cause = cause
	return err
}

func InvalidInput(message string, details map[string]any) error {
	return NewError(ErrorInvalidInput, message, details)
}

func InvalidPlan(message string, details map[string]any) error {
	return NewError(ErrorInvalidPlan, message, details)
}

func EventConflict(message string, details map[string]any) error {
	return NewError(ErrorEventConflict, message, details)
}

func Errorf(code, format string, args ...any) error {
	return NewError(code, fmt.Sprintf(format, args...), nil)
}
