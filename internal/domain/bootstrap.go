package domain

import "errors"

var (
	ErrBootstrapInput       = errors.New("invalid bootstrap credentials")
	ErrBootstrapInitialized = errors.New("administrator already initialized")
)
