// Package apns provide a Apple Push Notification service provider.
//
// Apple Push Notification service (APNs) is the centerpiece of the remote
// notifications feature. It is a robust and highly efficient service for
// propagating information to iOS (and, indirectly, watchOS), tvOS, and macOS
// devices. On initial activation, a device establishes an accredited and
// encrypted IP connection with APNs and receives notifications over this
// persistent connection. If a notification for an app arrives when that app is
// not running, the device alerts the user that the app has data waiting for it.
//
// You provide your own server to generate remote notifications for the users of
// your app. This server, known as the provider, has three main
// responsibilities. It:
//	- Gathers data for the users of your app
//	- Decides when push notifications need to be sent and determines to which
//	  devices they should be sent
//	- Sends notifications, which APNs conveys on your behalf to users
//
// For each notification, the provider:
//	1. Generates a notification payload
//	2. Attaches the payload and a device identifier — the device token — to an
// 	   HTTP/2 request
//	3. Sends the request to APNs over a persistent and secure channel that uses
//	   the HTTP/2 network protocol
//
// On receiving the HTTP/2 request, APNs delivers the notification payload to
// your app on the user's device.
//
// Quality of Service
//
// Apple Push Notification service includes a default Quality of Service (QoS)
// component that performs a store-and-forward function. If APNs attempts to
// deliver a notification but the destination device is offline, APNs stores the
// notification for a limited period of time and delivers it to the device when
// the device becomes available.
//
// This mechanism stores only one recent notification per device, per app: if
// you send multiple notifications while a device is offline, a new notification
// causes the previous notification to be discarded.
//
// If a device remains offline for a long time, all notifications that were
// being stored for it are discarded; when the device goes back online, none of
// the notifications are displayed.
//
// When a device is online, all the notifications you send are delivered and
// available to the user. However, you can avoid showing duplicate notifications
// by employing a collapse identifier across multiple, identical notifications.
// The APNs request header key for the collapse identifier is
// "apns-collapse-id".
//
// For example, a news service that sends the same headline twice in a row could
// employ the same collapse identifier for both push notification requests. APNs
// would then take care of coalescing these requests into a single notification
// for delivery to a device.
//
// Security Architecture
//
// To ensure secure communication, APNs servers employ connection certificates,
// certification authority (CA) certificates, and cryptographic keys (private
// and public) to validate connections to, and identities of, providers and
// devices. APNs regulates the entry points between providers and devices using
// two levels of trust: connection trust and device token trust.
//
// Connection trust establishes certainty that APNs is connected to an
// authorized provider, owned by a company that Apple has agreed to deliver
// notifications for. You must take steps to ensure connection trust exists
// between your provider servers and APNs. APNs also uses connection trust with
// each device to ensure the legitimacy of the device. Connection trust with the
// device is handled automatically by APNs.
//
// Device token trust ensures that notifications are routed only between
// legitimate start and end points. A device token is an opaque, unique
// identifier assigned to a specific app on a specific device. Each app instance
// receives its unique token when it registers with APNs. The app must share
// this token with its provider, to allow the provider to employ the token when
// communicating with APNs. Each notification that your provider sends to APNs
// must include the device token, which ensures that the notification is
// delivered only to the app-device combination for which it is intended.
//
// Important: To protect user privacy, do not attempt to use a device token to
// identify a device.
//
// Device tokens can change after updating the operating system, and always
// change when a device's data and settings are erased. Whenever the system
// delivers a device token to an instance of your app, the app must forward it
// to your provider servers to allow further push notifications to the device.
//
// Provider-to-APNs Connection Trust
//
// A provider using the HTTP/2-based APNs Provider API can use JSON web tokens
// (JWT) to validate the provider's connection with APNs. In this scheme, the
// provider does not require a certificate-plus-private key to establish
// connection. Instead, you provision a public key to be retained by Apple, and
// a private key which you retain and protect. Your providers then use your
// private key to generate and sign JWT authentication tokens. Each of your push
// requests must include an authentication token.
//
// Important: To establish TLS sessions with APNs, you must ensure that a
// GeoTrust Global CA root certificate is installed on each of your providers.
// You can download this certificate from the GeoTrust Root Certificates
// website: https://www.geotrust.com/resources/root-certificates/.
//
// The HTTP/2-based provider connection is valid for delivery to one specific
// app, identified by the topic (the app bundle ID) specified in the
// certificate. Depending on how you configure and provision your APNs Transport
// Layer Security (TLS) certificate, the trusted connection can also be valid
// for delivery of remote notifications to other items associated with your app,
// including Apple Watch complications and voice-over-Internet Protocol (VoIP)
// services. APNs delivers these notifications even when those items are running
// in the background.
//
// APNs maintains a certificate revocation list; if a provider's certificate is
// on the revocation list, APNs can revoke provider trust (that is, APNs can
// refuse the TLS initiation connection).
package apns
