package client

import (
	"fmt"
	"net/http"
)

type Error struct {
	StatusCode   int
	Body         []byte
	Err          error
	Retries      int
	Method       string
	URL          string
	LastResponse *http.Response
}

func (e *Error) Error() string {
	msg := fmt.Sprintf("[HTTP] %s %s: status=%d, err=%v", e.Method, e.URL, e.StatusCode, e.Err)
	if len(e.Body) > 0 {
		msg += fmt.Sprintf(", body=%s", string(e.Body))
	}
	return msg
}
