package service

import "github.com/RhinoLink-Tech/data-fabric/internal/repository"

// The service layer returns the repository's errors, some of them wrapped with
// %w. These are aliases rather than separate values so that a caller comparing
// against them matches the error the repository actually returns; compare with
// errors.Is, which also sees through the wrapping.
var (
	ErrDataSourceNotFound  = repository.ErrDataSourceNotFound
	ErrDataSourceExists    = repository.ErrDataSourceExists
	ErrInvalidUUID         = repository.ErrInvalidUUID
	ErrInvalidDatabaseType = repository.ErrInvalidDatabaseType
	ErrConnectionFailed    = repository.ErrConnectionFailed
)
