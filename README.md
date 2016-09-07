# Apple Push Notification Service Provider (HTTP/2)

[![GoDoc](https://godoc.org/github.com/mdigger/apns?status.svg)](https://godoc.org/github.com/mdigger/apns)
[![Build Status](https://travis-ci.org/mdigger/apns.svg)](https://travis-ci.org/mdigger/apns)
[![Coverage Status](https://coveralls.io/repos/github/mdigger/apns/badge.svg?branch=master)](https://coveralls.io/github/mdigger/apns?branch=master)

Apple Push Notification service includes the APNs Provider API that allows you to send remote notifications to your app on iOS, tvOS, and OS X devices, and to Apple Watch via iOS. This API is based on the HTTP/2 network protocol. Each interaction starts with a POST request, containing a JSON payload, that you send from your provider server to APNs. APNs then forwards the notification to your app on a specific user device.

Your APNs certificate, which you obtain as explained in Creating a Universal Push Notification Client SSL Certificate in App Distribution Guide, enables connection to both the APNs Production and Development environments.

You can use your APNs certificate to send notifications to your primary app, as identified by its bundle ID, as well as to any Apple Watch complications or backgrounded VoIP services associated with that app.

```go
package main

import (
	"log"
	"math/rand"
	"time"

	"github.com/mdigger/apns"
)

func main() {
	cert, err := apns.LoadCertificate("cert.p12", "xopen123")
	if err != nil {
		log.Fatalln("Error loading certificate:", err)
	}
	client := apns.New(*cert)
	id, err := client.Push(apns.Notification{
		Token: `883982D57CDC4138D71E16B5ACBCB5DEBE3E625AFCEEE809A0F32895D2EA9D51`,
		Payload: map[string]interface{}{
			"aps": map[string]interface{}{
				"alert": "Hello!",
				"badge": rand.Int31n(99),
			},
			"time": time.Now().Format(time.RFC3339Nano),
		},
	})
	if err != nil {
		log.Fatalln("Error push:", err)
	}
	log.Println("Sent:", id)
}
```
