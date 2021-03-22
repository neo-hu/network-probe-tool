package network

import "errors"

var (
	ErrAlreadyRunning = errors.New("already running")
	ErrAlreadyClosed  = errors.New("already closed")
	ErrNotRunning     = errors.New("not running")
)