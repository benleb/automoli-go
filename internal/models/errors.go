package models

import "errors"

var (
	// general errors.
	ErrEmptyURL   = errors.New("URL cannot be empty")
	ErrEmptyToken = errors.New("token cannot be empty")

	// connection errors.
	ErrNoConnectionToReadFrom = errors.New("no connection to read from")
	ErrNoConnectionToWriteTo  = errors.New("no connection to write to")
	ErrConnectionClosed       = errors.New("connection closed")

	// home assistant errors.
	ErrNoStatesReceived      = errors.New("no states received")
	ErrUnexpectedMessageType = errors.New("unexpected message type")
)
