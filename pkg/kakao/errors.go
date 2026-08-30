package kakao

import "fmt"

// APIError represents an error response from Kakao API.
type APIError struct {
	Msg              string `json:"msg,omitempty"`
	Code             int    `json:"code,omitempty"`
	ErrorStr         string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
}

func (e *APIError) Error() string {
	if e.ErrorDescription != "" {
		return fmt.Sprintf("kakao error (%s): %s", e.ErrorStr, e.ErrorDescription)
	}
	if e.Msg != "" {
		return fmt.Sprintf("kakao error [%d]: %s", e.Code, e.Msg)
	}
	if e.ErrorStr != "" {
		return fmt.Sprintf("kakao error: %s", e.ErrorStr)
	}
	return "unknown kakao api error"
}
