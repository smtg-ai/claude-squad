package session

import "errors"

var (
	ErrInstanceNotStarted    = errors.New("instance not started")
	ErrInstanceAlreadyPaused = errors.New("instance is already paused")
	ErrBranchCheckedOut      = errors.New("branch is currently checked out")
	ErrTitleEmpty            = errors.New("instance title cannot be empty")
	ErrTitleImmutable        = errors.New("cannot change title of a started instance")
)
