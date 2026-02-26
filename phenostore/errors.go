package phenostore

import (
	"encoding/json"
	"errors"
	"fmt"
)

// OperationOutcomeError represents an error response from the Phenostore API
// containing a FHIR OperationOutcome.
type OperationOutcomeError struct {
	StatusCode int
	Body       []byte
}

func (e *OperationOutcomeError) Error() string {
	var parsed struct {
		Issue []struct {
			Severity    string `json:"severity"`
			Code        string `json:"code"`
			Diagnostics string `json:"diagnostics"`
		} `json:"issue"`
	}
	if json.Unmarshal(e.Body, &parsed) == nil && len(parsed.Issue) > 0 {
		issue := parsed.Issue[0]
		return fmt.Sprintf("[%s/%s] %s", issue.Severity, issue.Code, issue.Diagnostics)
	}
	return fmt.Sprintf("phenostore: HTTP %d", e.StatusCode)
}

// IsNotFound returns true if the error is a 404 Not Found.
func IsNotFound(err error) bool {
	var ooe *OperationOutcomeError
	return errors.As(err, &ooe) && ooe.StatusCode == 404
}

// IsGone returns true if the error is a 410 Gone.
func IsGone(err error) bool {
	var ooe *OperationOutcomeError
	return errors.As(err, &ooe) && ooe.StatusCode == 410
}

// IsConflict returns true if the error is a 412 Precondition Failed.
func IsConflict(err error) bool {
	var ooe *OperationOutcomeError
	return errors.As(err, &ooe) && ooe.StatusCode == 412
}

// IsForbidden returns true if the error is a 403 Forbidden.
func IsForbidden(err error) bool {
	var ooe *OperationOutcomeError
	return errors.As(err, &ooe) && ooe.StatusCode == 403
}
