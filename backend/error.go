package backend

import (
	"errors"
)

type ServerError error

var (
	DatabaseError = ServerError(errors.New("database error"))
)
