package apns

import (
	"encoding/json"
	"io"
	"net/http"
	"time"
)

func decodeError(status int, body io.Reader) error {
	var response = &Error{Status: status}
	if err := json.NewDecoder(body).Decode(response); err != nil {
		return err
	}
	return response
}

// Error describes the error response from the server.
type Error struct {
	// The HTTP status code:
	// 	400 - Bad request
	//  403 - There was an error with the certificate.
	// 	405 - The request used a bad :method value. Only POST requests are supported.
	// 	410 - The device token is no longer active for the topic.
	// 	413 - The notification payload was too large.
	//  429 - The server received too many requests for the same device token.
	// 	500 - Internal server error
	// 	503 - The server is shutting down and unavailable.
	Status int

	// The error indicating the reason for the failure.
	Reason string `json:"reason"`

	// If the value in the Status is 410, the value of this key is the last time
	// at which APNs confirmed that the device token was no longer valid for
	// the topic.
	//
	// Stop pushing notifications until the device registers a token with a
	// later timestamp with your provider.
	Timestamp int64 `json:"timestamp"`
}

// Error return full error description string.
func (e *Error) Error() string {
	msg, ok := reasons[e.Reason]
	if !ok {
		if msg = http.StatusText(e.Status); msg == "" {
			msg = e.Reason
		}
	}
	return msg
}

// Time return parsed time and date, returned from server with response.
// If the value in the Status is 410, the returned value is the last time
// at which APNs confirmed that the device token was no longer valid for
// the topic.
func (e *Error) Time() time.Time {
	if e.Timestamp == 0 {
		return time.Time{}
	}
	return time.Unix(e.Timestamp/1000, 0)
}

// IsToken returns true if the error associated with the device token.
func (e *Error) IsToken() bool {
	switch e.Reason {
	case "MissingDeviceToken", "BadDeviceToken", "DeviceTokenNotForTopic",
		"Unregistered":
		return true
	}
	return false
}

var reasons = map[string]string{
	"PayloadEmpty":              "The message payload was empty.", // 400
	"PayloadTooLarge":           "The message payload was too large. The maximum payload size is 4096 bytes.",
	"BadTopic":                  "The apns-topic was invalid.",
	"TopicDisallowed":           "Pushing to this topic is not allowed.",
	"BadMessageId":              "The apns-id value is bad.",
	"BadExpirationDate":         "The apns-expiration value is bad.",
	"BadPriority":               "The apns-priority value is bad.",
	"MissingDeviceToken":        "The device token is not specified in the request :path. Verify that the :path header contains the device token.",
	"BadDeviceToken":            "The specified device token was bad. Verify that the request contains a valid token and that the token matches the environment.",
	"DeviceTokenNotForTopic":    "The device token does not match the specified topic.",
	"Unregistered":              "The device token is inactive for the specified topic.", // 410
	"DuplicateHeaders":          "One or more headers were repeated.",
	"BadCertificateEnvironment": "The client certificate was for the wrong environment.",
	"BadCertificate":            "The certificate was bad.",
	"Forbidden":                 "The specified action is not allowed.",
	"BadPath":                   "The request contained a bad :path value.",
	"MethodNotAllowed":          "The specified :method was not POST.",
	"TooManyRequests":           "Too many requests were made consecutively to the same device token.",
	"IdleTimeout":               "Idle time out.",
	"Shutdown":                  "The server is shutting down.",
	"InternalServerError":       "An internal server error occurred.",
	"ServiceUnavailable":        "The service is unavailable.",
	"MissingTopic":              "The apns-topic header of the request was not specified and was required. The apns-topic header is mandatory when the client is connected using a certificate that supports multiple topics",
}
