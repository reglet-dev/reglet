package domain

// Status represents the status of a control or observation.
type Status string

const (
	// StatusPass indicates the check passed
	StatusPass Status = "pass"
	// StatusFail indicates the check failed (but ran successfully)
	StatusFail Status = "fail"
	// StatusError indicates the check encountered an error
	StatusError Status = "error"
	// StatusSkipped indicates the check was skipped (dependency failure or filtered)
	StatusSkipped Status = "skipped"
)
