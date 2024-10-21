package backend

import "fmt"

type ServerError error

func DatabaseError(message string) ServerError {
	return ServerError(fmt.Errorf("Database error: %s", message))
}
