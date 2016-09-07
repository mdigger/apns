package apns_test

import (
	"log"
	"math/rand"
	"time"

	"github.com/mdigger/apns"
)

func Example() {
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
