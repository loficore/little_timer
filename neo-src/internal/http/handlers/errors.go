package handlers

// handlerError is a tiny error type used by the pathID helpers.
type handlerError struct {
	message string
}

func (e *handlerError) Error() string { return e.message }