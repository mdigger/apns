package apns

import (
	"bytes"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/http2"
)

// Client describes APNS client service to send notifications to devices.
type Client struct {
	*CertificateInfo              // certificate info
	Sandbox          bool         // sandbox flag
	httpСlient       *http.Client // http client for push
}

// New initialize and return the APNS client to send notifications.
func New(certificate tls.Certificate) *Client {
	var tlsConfig = &tls.Config{
		Certificates: []tls.Certificate{certificate}}
	var transport = &http.Transport{TLSClientConfig: tlsConfig}
	if err := http2.ConfigureTransport(transport); err != nil {
		panic(err) // HTTP/2 initialization error
	}
	return &Client{
		CertificateInfo: GetCertificateInfo(certificate),
		httpСlient: &http.Client{
			Timeout:   15 * time.Second,
			Transport: transport,
		},
	}
}

// Notification describes the information for sending Apple Push.
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
	// multiple topics, the APNs server uses the certificate’s Subject as the
	// default topic.
	Topic string

	// The body content of your message is the JSON dictionary object
	// containing the notification data.
	Payload interface{}
}

// Push sends a notification to the Apple server.
//
// Return the ID value from the notification. If no value was included in the
// notification, the server creates a new UUID and returns it.
func (c *Client) Push(n Notification) (id string, err error) {
	var payload []byte
	switch data := n.Payload.(type) {
	case []byte:
		payload = data
	case string:
		payload = []byte(data)
	case json.RawMessage:
		payload = []byte(data)
	default:
		if payload, err = json.Marshal(n.Payload); err != nil {
			return "", err
		}
	}
	if len(payload) > 4096 {
		return "", &Error{
			Status: http.StatusRequestEntityTooLarge,
			Reason: "PayloadTooLarge",
		}
	}
	// check token format and length
	if l := len(n.Token); l < 64 || l > 200 {
		return "", &Error{
			Status: http.StatusBadRequest,
			Reason: "BadDeviceToken",
		}
	}
	if _, err = hex.DecodeString(n.Token); err != nil {
		return "", &Error{
			Status: http.StatusBadRequest,
			Reason: "BadDeviceToken",
		}
	}
	var host = "https://api.push.apple.com"
	if !c.CertificateInfo.Production ||
		(c.Sandbox && c.CertificateInfo.Development) {
		host = "https://api.development.push.apple.com"
	}
	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%v/3/device/%v", host, n.Token), bytes.NewReader(payload))
	if err != nil {
		return "", err
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
	if len(c.CertificateInfo.Topics) > 0 {
		if n.Topic == "" {
			n.Topic = c.CertificateInfo.BundleID
		}
		req.Header.Set("apns-topic", n.Topic)
	}
	resp, err := c.httpСlient.Do(req)
	if err != nil {
		if err, ok := err.(*url.Error); ok {
			if err, ok := err.Err.(http2.GoAwayError); ok {
				return "", decodeError(0, strings.NewReader(err.DebugData))
			}
		}
		return "", err
	}
	defer resp.Body.Close()
	// defer func() {
	// 	io.CopyN(ioutil.Discard, resp.Body, 2<<10)
	// 	resp.Body.Close()
	// }()
	id = resp.Header.Get("apns-id")
	if resp.StatusCode != http.StatusOK {
		return id, decodeError(resp.StatusCode, resp.Body)
	}
	return id, nil
}
