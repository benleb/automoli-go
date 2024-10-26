package models

import (
	"errors"
	"fmt"
)

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
	ErrEmptyEntityID         = errors.New("empty entity id")
	ErrInvalidEntityID       = errors.New("invalid entity id")

	// light conditions.
	ErrLightAlreadyOn    = errors.New("light is already on")
	ErrLightJustTurnedOn = errors.New("light just turned on")
	// ErrLightAlreadyOff   = errors.New("light is already off").
	ErrAutoMoLiDisabled = errors.New("AutoMoLi is disabled")
	ErrDaytimeDisabled  = errors.New("disabled by light configuration for this daytime")
)

func InvalidEntityIDErr(rawEntityID string) error {
	return fmt.Errorf("%w: %s", ErrInvalidEntityID, rawEntityID)
}

func EmptyEntityIDErr() error {
	return fmt.Errorf("%w", ErrEmptyEntityID)
}
