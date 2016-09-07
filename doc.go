// Package apns implements Apple Push Notification Protocol.
//
// Apple Push Notification service includes the APNs Provider API that allows
// you to send remote notifications to your app on iOS, tvOS, and OS X devices,
// and to Apple Watch via iOS. This API is based on the HTTP/2 network protocol.
// Each interaction starts with a POST request, containing a JSON payload, that
// you send from your provider server to APNs. APNs then forwards the
// notification to your app on a specific user device.
//
// Your APNs certificate, which you obtain as explained in Creating a Universal
// Push Notification Client SSL Certificate in App Distribution Guide, enables
// connection to both the APNs Production and Development environments.
//
// You can use your APNs certificate to send notifications to your primary app,
// as identified by its bundle ID, as well as to any Apple Watch complications
// or backgrounded VoIP services associated with that app.
package apns
