package reports

import "errors"

var (
	// ErrInvalidRange means "to" is before "from".
	ErrInvalidRange = errors.New("to must not be before from")
	// ErrInvalidMonths means the requested trend window was out of bounds.
	ErrInvalidMonths = errors.New("months must be between 1 and 24")
)
