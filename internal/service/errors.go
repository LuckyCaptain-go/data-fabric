package service

import (
	"errors"

	"github.com/RhinoLink-Tech/data-fabric/internal/repository"
)

// Repository errors are aliases so errors.Is matches the repository values
// through service-layer wrapping. Validation sentinels identify client errors
// before a request reaches persistence.
var (
	ErrDataSourceNotFound        = repository.ErrDataSourceNotFound
	ErrDataSourceExists          = repository.ErrDataSourceExists
	ErrInvalidUUID               = repository.ErrInvalidUUID
	ErrInvalidDatabaseType       = repository.ErrInvalidDatabaseType
	ErrConnectionFailed          = repository.ErrConnectionFailed
	ErrUnsupportedDataSourceType = errors.New("unsupported data source type")
	ErrInvalidDataSourceConfig   = errors.New("invalid data source configuration")
)
