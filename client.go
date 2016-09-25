package apns

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/http2"
)

// Timeout contains the maximum waiting time connection to the APNS server.
var Timeout = 15 * time.Second

// Client supports APNs Provider API.
//
// The APNs provider API lets you send remote notifications to your app on iOS,
// tvOS, and macOS devices, and to Apple Watch via iOS. The API is based on the
// HTTP/2 network protocol. Each interaction starts with a POST request,
// containing a JSON payload, that you send from your provider server to APNs.
// APNs then forwards the notification to your app on a specific user device.
//
// The first step in sending a remote notification is to establish a connection
// with the appropriate APNs server Host:
// 	Development server: api.development.push.apple.com:443
// 	Production server:  api.push.apple.com:443
//
// Note: You can alternatively use port 2197 when communicating with APNs. You
// might do this, for example, to allow APNs traffic through your firewall but
// to block other HTTPS traffic.
//
// The APNs server allows multiple concurrent streams for each connection. The
// exact number of streams is based on the authentication method used (i.e.
// provider certificate or token) and the server load, so do not assume a
// specific number of streams. When you connect to APNs without a provider
// certificate, only one stream is allowed on the connection until you send a
// push message with valid token.
//
// It is recommended to close all existing connections to APNs and open new
// connections when existing certificate or the key used to sign provider tokens
// is revoked.
type Client struct {
	Host       string           // http URL
	ci         *CertificateInfo // certificate
	token      *ProviderToken   // provider token
	httpСlient *http.Client     // http client for push
}

func newClient(certificate *tls.Certificate, pt *ProviderToken) *Client {
	client := &Client{
		Host:       "https://api.push.apple.com",
		httpСlient: &http.Client{Timeout: Timeout},
	}
	if pt != nil {
		client.token = pt
	}
	if certificate != nil {
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates: []tls.Certificate{*certificate}}}
		if err := http2.ConfigureTransport(transport); err != nil {
			panic(err) // HTTP/2 initialization error
		}
		client.httpСlient.Transport = transport
		client.ci = GetCertificateInfo(certificate)
		if !client.ci.Production {
			client.Host = "https://api.development.push.apple.com"
		}
	}
	return client
}

func New(certificate tls.Certificate) *Client {
	return newClient(&certificate, nil)
}

// NewWithToken returns an initialized Client with JSON Web Token (JWT)
// authentication support.
func NewWithToken(pt *ProviderToken) *Client {
	return newClient(nil, pt)
}

// Push send push notification to APNS API.
//
// The APNs Provider API consists of a request and a response that you configure
// and send using an HTTP/2 POST command. You use the request to send a push
// notification to the APNs server and use the response to determine the results
// of that request.
//
// Response from APNs:
// 	- The apns-id value from the request. If no value was included in the
//	  request, the server creates a new UUID and returns it in this header.
// 	- :status - the HTTP status code.
//	- reason - the error indicating the reason for the failure. The error code
// 	  is specified as a string.
//	- timestamp - if the value in the :status header is 410, the value of this
//	  key is the last time at which APNs confirmed that the device token was no
//	  longer valid for the topic. Stop pushing notifications until the device
//	  registers a token with a later timestamp with your provider.
func (c *Client) Push(notification Notification) (id string, err error) {
	req, err := notification.request(c.Host)
	if err != nil {
		return "", err
	}
	req.Header.Set("user-agent", "mdigger-apns/3.1")
	// add default certificate topic
	if notification.Topic == "" && c.ci != nil && len(c.ci.Topics) > 0 {
		// If your certificate includes multiple topics, you must specify a
		// value for this header.
		req.Header.Set("apns-topic", c.ci.BundleID)
	}
	if c.token != nil {
		// The provider token that authorizes APNs to send push notifications
		// for the specified topics. The token is in Base64URL-encoded JWT
		// format, specified as bearer <provider token>.
		// When the provider certificate is used to establish a connection, this
		// request header is ignored.
		if token, err := c.token.JWT(); err == nil {
			req.Header.Set("authorization", fmt.Sprintf("bearer %s", token))
		}
	}

	resp, err := c.httpСlient.Do(req)
	if err, ok := err.(*url.Error); ok {
		// If APNs decides to terminate an established HTTP/2 connection, it
		// sends a GOAWAY frame. The GOAWAY frame includes JSON data in its
		// payload with a reason key, whose value indicates the reason for the
		// connection termination.
		if err, ok := err.Err.(http2.GoAwayError); ok {
			return "", parseError(0, strings.NewReader(err.DebugData))
		}
	}
	if err != nil {
		return "", err
	}
	// For a successful request, the body of the response is empty. On failure,
	// the response body contains a JSON dictionary.
	defer resp.Body.Close()
	id = resp.Header.Get("apns-id")
	if resp.StatusCode == http.StatusOK {
		return id, nil
	}
	return id, parseError(resp.StatusCode, resp.Body)
}
