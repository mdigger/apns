package apns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// Notification describes the Remote Notification for sending to Apple Push.
//
// Each Apple Push Notification service (APNs) remote notification includes a
// payload. The payload contains the custom data you want to provide, along with
// information about how the system should alert the user.
//
// The maximum payload size for a notification: for Regular push notification is
// 4KB (4096 bytes), for Voice over Internet Protocol (VoIP) notification — 5KB
// (5120 bytes).
//
// APNs refuses notifications that exceed the maximum size.
//
// For each notification, compose a JSON dictionary object (as defined by RFC
// 4627). This dictionary must contain another dictionary identified by the aps
// key. The aps dictionary can contain one or more properties that specify the
// following user notification types: an alert message to display to the user,
// a number to badge the app icon with and a sound to play.
//
// If the target app isn't running when the notification arrives, the alert
// message, sound, or badge value is played or shown. If the app is running, the
// system delivers the notification to the app.
//
// Providers can specify custom payload values outside the Apple-reserved aps
// dictionary. Custom values must use the JSON structured and primitive types:
// dictionary (object), array, string, number, and Boolean. You should not
// include customer information (or any sensitive data) as custom payload data.
// Instead, use it for such purposes as setting context (for the user interface)
// or internal metrics. For example, a custom payload value might be a
// conversation identifier for use by an instant-message client app or a
// timestamp identifying when the provider sent the notification. Any action
// associated with an alert message should not be destructive—for example, it
// should not delete data on the device.
//
// Important: Delivery of notifications is a “best effort”, not guaranteed. It
// is not intended to deliver data to your app, only to notify the user that
// there is new data available.
//
// The aps dictionary can also contain the content-available property. The
// content-available property with a value of 1 lets the remote notification act
// as a silent notification. When a silent notification arrives, iOS wakes up
// your app in the background so that you can get new data from your server or
// do background information processing. Users aren't told about the new or
// changed information that results from a silent notification, but they can
// find out about it the next time they open your app.
//
// For a silent notification, take care to ensure there is no alert, sound, or
// badge payload in the aps dictionary. If you don't follow this guidance, the
// incorrectly-configured notification might be throttled and not delivered to
// the app in the background, and instead of being silent is displayed to the
// user.
type Notification struct {
	// Unique device token for the app.
	//
	// Every notification that your provider sends to APNs must be accompanied
	// by the device token associated of the device for which the notification
	// is intended.
	Token string

	// A canonical UUID that identifies the notification.  If there is an error
	// sending the notification, APNs uses this value to identify the
	// notification to your server.
	//
	// The canonical form is 32 lowercase hexadecimal digits, displayed in five
	// groups separated by hyphens in the form 8-4-4-4-12. An example UUID is
	// as follows: 123e4567-e89b-12d3-a456-42665544000
	//
	// If you omit this header, a new UUID is created by APNs and returned in
	// the response.
	ID string

	// This identifies the date when the notification is no longer valid and
	// can be discarded.
	//
	// If this value is in future time, APNs stores the notification and tries
	// to deliver it at least once, repeating the attempt as needed if it is
	// unable to deliver the notification the first time. If the value is
	// before now, APNs treats the notification as if it expires immediately
	// and does not store the notification or attempt to redeliver it.
	Expiration time.Time

	// Specify the hexadecimal bytes (hex-string) of the device token for the
	// target device.
	//
	// Flag for send the push message at a time that takes into account power
	// considerations for the device. Notifications with this priority might be
	// grouped and delivered in bursts. They are throttled, and in some cases
	// are not delivered.
	LowPriority bool

	// The topic of the remote notification, which is typically the bundle ID
	// for your app. The certificate you create in Member Center must include
	// the capability for this topic.
	//
	// If your certificate includes multiple topics, you can specify a value
	// for this. If you omit this or your APNs certificate does not specify
	// multiple topics, the APNs server uses the certificate's Subject as the
	// default topic.
	//
	// If you are using a provider token instead of a certificate, you must
	// specify a value for this request header. The topic you provide should be
	// provisioned for the your team named in your developer account.
	Topic string

	// Multiple notifications with same collapse identifier are displayed to the
	// user as a single notification. The value should not exceed 64 bytes.
	CollapseID string

	// The body content of your message is the JSON dictionary object containing
	// the notification data. The body data must not be compressed and its
	// maximum size is 4KB (4096 bytes). For a Voice over Internet Protocol
	// (VoIP) notification, the body data maximum size is 5KB (5120 bytes).
	Payload interface{}
}

// request return HTTP/2 Request to APNs
//
// Use a request to send a notification to a specific user device.
// 	:method POST
// 	:path /3/device/<device-token>
// For the <device-token> parameter, specify the hexadecimal bytes of the device
// token for the target device.
//
// APNs requires the use of HPACK (header compression for HTTP/2), which
// prevents repeated header keys and values. APNs maintains a small dynamic
// table for HPACK. To help avoid filling up the APNs HPACK table and
// necessitating the discarding of table data, encode headers in the following
// way—especially when sending a large number of streams:
// 	- The :path value should be encoded as a literal header field without
//	  indexing
// 	- The authorization request header, if present, should be encoded as a
//	  literal header field without indexing
// 	- The appropriate encoding to employ for the apns-id, apns-expiration, and
//	  apns-collapse-id request headers differs depending on whether it is part
// 	  of the initial or a subsequent POST operation, as follows: the first time
// 	  you send these headers, encode them with incremental indexing to allow the
// 	  header names to be added to the dynamic table; subsequent times you send
// 	  these headers, encode them as literal header fields without indexing.
// Encode all other headers as literal header fields with incremental indexing.
//
// The body content of your message is the JSON dictionary object containing the
// notification data. The body data must not be compressed and its maximum size
// is 4KB (4096 bytes). For a Voice over Internet Protocol (VoIP) notification,
// the body data maximum size is 5KB (5120 bytes).
func (n *Notification) request(host string) (req *http.Request, err error) {
	var payload []byte
	switch data := n.Payload.(type) {
	case []byte:
		payload = data
	case string:
		payload = []byte(data)
	case json.RawMessage:
		payload = []byte(data)
	default:
		payload, err = json.Marshal(n.Payload)
		if err != nil {
			return nil, err
		}
	}
	req, err = http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/3/device/%s", host, n.Token), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if n.ID != "" {
		req.Header.Set("apns-id", n.ID)
	}
	if !n.Expiration.IsZero() {
		var exp string = "0"
		if !n.Expiration.Before(time.Now()) {
			exp = strconv.FormatInt(n.Expiration.Unix(), 10)
		}
		req.Header.Set("apns-expiration", exp)
	}
	if n.LowPriority {
		req.Header.Set("apns-priority", "5")
	}
	if n.Topic != "" {
		req.Header.Set("apns-topic", n.Topic)
	}
	if n.CollapseID != "" && len(n.CollapseID) <= 64 {
		req.Header.Set("apns-collapse-id", n.CollapseID)
	}
	return req, nil
}
