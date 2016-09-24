package apns

import (
	"encoding/json"
	"io"
	"net/http"
	"time"
)

func parseError(status int, body io.Reader) error {
	var response = &Error{Status: status}
	if err := json.NewDecoder(body).Decode(response); err != nil {
		return err
	}
	return response
}

// Error describes the error response from the server.
type Error struct {
	// List of the possible status codes for a request (these values are
	// included in the :status header of the response):
	// 	400 Bad request
	// 	403 There was an error with the certificate.
	// 	405 The request used a bad :method value. Only POST requests are
	// 	    supported.
	// 	410 The device token is no longer active for the topic.
	// 	413 The notification payload was too large.
	// 	429 The server received too many requests for the same device token.
	// 	500 Internal server error
	// 	503 The server is shutting down and unavailable.
	Status int

	// The error indicating the reason for the failure.
	//
	// List of the possible error codes included in the reason key of a
	// response's JSON payload:
	// 	400 BadCollapseId - the collapse identifier exceeds the maximum allowed
	// 	    size.
	// 	400 BadDeviceToken - the specified device token was bad. Verify that the
	// 	    request contains a valid token and that the token matches the
	// 	    environment.
	// 	400 BadExpirationDate - the apns-expiration value is bad.
	// 	400 BadMessageId - the apns-id value is bad.
	// 	400 BadPriority - the apns-priority value is bad.
	// 	400 BadTopic - the apns-topic was invalid.
	// 	400 DeviceTokenNotForTopic - the device token does not match the
	// 	    specified topic.
	// 	400 DuplicateHeaders - one or more headers were repeated.
	// 	400 IdleTimeout - idle time out.
	// 	400 MissingDeviceToken - the device token is not specified in the
	// 	    request :path. Verify that the :path header contains the device
	// 	    token.
	// 	400 MissingTopic - the apns-topic header of the request was not
	// 	    specified and was required. The apns-topic header is mandatory when
	// 	    the client is connected using a certificate that supports multiple
	// 	    topics.
	// 	400 PayloadEmpty - the message payload was empty. Expected HTTP/2
	// 	    :status code is 400.
	// 	400 TopicDisallowed - pushing to this topic is not allowed.
	// 	403 BadCertificate - the certificate was bad.
	// 	403 BadCertificateEnvironment - the client certificate was for the wrong
	// 	    environment.
	// 	403 ExpiredProviderToken - the provider token is stale and a new token
	// 	    should be generated.
	// 	403 Forbidden - the specified action is not allowed.
	// 	403 InvalidProviderToken - the provider token is not valid or the token
	// 	    signature could not be verified
	// 	403 MissingProviderToken - no provider certificate was used to connect
	// 	    to APNs and Authorization header was missing or no provider token
	// 	    was specified
	// 	404 BadPath - the request contained a bad :path value.
	// 	405 MethodNotAllowed - the specified :method was not POST.
	// 	410 Unregistered - the device token is inactive for the specified topic.
	// 	    Expected HTTP/2 status code is 410; see Table 6-4.
	// 	413 PayloadTooLarge - the message payload was too large. See The Remote
	// 	    Notification Payload for details on maximum payload size.
	// 	429 TooManyProviderTokenUpdates - the provider token is being updated
	// 	    too often.
	// 	429 TooManyRequests - too many requests were made consecutively to the
	// 	    same device token.
	// 	500 InternalServerError - an internal server error occurred.
	// 	503 ServiceUnavailable - the service is unavailable.
	// 	503 Shutdown - the server is shutting down.
	Reason string `json:"reason"`

	// If the value in the :status header is 410, the value of this key is the
	// last time at which APNs confirmed that the device token was no longer
	// valid for the topic. Stop pushing notifications until the device
	// registers a token with a later timestamp with your provider.
	Timestamp int64 `json:"timestamp"`
}

// Error return full error description string.
func (e *Error) Error() string {
	msg, ok := reasons[e.Reason]
	if ok {
		return msg
	}
	msg = http.StatusText(e.Status)
	if msg == "" {
		msg = e.Reason
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
	case "MissingDeviceToken",
		"BadDeviceToken",
		"DeviceTokenNotForTopic",
		"Unregistered":
		return true
	default:
		return false
	}
}

// List of the possible error codes included in the reason key of a response's
// JSON payload:
var reasons = map[string]string{
	"BadCollapseId":               "The collapse identifier exceeds the maximum allowed size.",
	"BadDeviceToken":              "The specified device token was bad. Verify that the request contains a valid token and that the token matches the environment.",
	"BadExpirationDate":           "The apns-expiration value is bad.",
	"BadMessageId":                "The apns-id value is bad.",
	"BadPriority":                 "The apns-priority value is bad.",
	"BadTopic":                    "The apns-topic was invalid.",
	"DeviceTokenNotForTopic":      "The device token does not match the specified topic.",
	"DuplicateHeaders":            "One or more headers were repeated.",
	"IdleTimeout":                 "Idle time out.",
	"MissingDeviceToken":          "The device token is not specified in the request :path. Verify that the :path header contains the device token.",
	"MissingTopic":                "The apns-topic header of the request was not specified and was required. The apns-topic header is mandatory when the client is connected using a certificate that supports multiple topics.",
	"PayloadEmpty":                "The message payload was empty. Expected HTTP/2 :status code is 400.",
	"TopicDisallowed":             "Pushing to this topic is not allowed.",
	"BadCertificate":              "The certificate was bad.",
	"BadCertificateEnvironment":   "The client certificate was for the wrong environment.",
	"ExpiredProviderToken":        "The provider token is stale and a new token should be generated.",
	"Forbidden":                   "The specified action is not allowed.",
	"InvalidProviderToken":        "The provider token is not valid or the token signature could not be verified.",
	"MissingProviderToken":        "No provider certificate was used to connect to APNs and Authorization header was missing or no provider token was specified.",
	"BadPath":                     "The request contained a bad :path value.",
	"MethodNotAllowed":            "The specified :method was not POST.",
	"Unregistered":                "The device token is inactive for the specified topic. Expected HTTP/2 status code is 410.",
	"PayloadTooLarge":             "The message payload was too large. See The Remote Notification Payload for details on maximum payload size.",
	"TooManyProviderTokenUpdates": "The provider token is being updated too often.",
	"TooManyRequests":             "Too many requests were made consecutively to the same device token.",
	"InternalServerError":         "An internal server error occurred.",
	"ServiceUnavailable":          "The service is unavailable.",
	"Shutdown":                    "The server is shutting down.",
}
