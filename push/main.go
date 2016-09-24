// Send Apple Push notification
//
//  ./push [-params] <token> [<token2> [...]]
//    -t    use development service
//    -b badge
//          badge number
//    -c certificate
//          push certificate (default "cert.p12")
//    -f file
//          JSON file with push message
//    -p password
//          certificate password
//    -a text
//          message text (default "Hello!")
//
//  Sample JSON file:
//    {
//      "payload": {
//        "aps": {
//          "alert": "message",
//          "badge": 0
//        }
//      }
//    }
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/mdigger/apns"
)

func main() {
	certFileName := flag.String("c", "cert.p12", "push `certificate`")
	password := flag.String("p", "", "certificate `password`")
	development := flag.Bool("t", false, "use sandbox service")
	notificationFileName := flag.String("f", "", "JSON `file` with push message")
	alert := flag.String("a", "Hello!", "message `text`")
	badge := flag.Uint("b", 0, "`badge` number")
	topic := flag.String("i", "", "`topic` id")
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, "Send Apple Push notification\n")
		fmt.Fprintf(os.Stderr, "%s [-params] <token> [<token2> [...]]\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\n"+`Sample JSON file:
  { 
    "payload": {
      "aps": {
        "alert": "message",
        "badge": 0 
      }
    }
  }`)
	}
	flag.Parse()
	log.SetFlags(0)

	if flag.NArg() < 1 {
		log.Fatalln("Error: no tokens")
	}
	tokens := flag.Args()
	var payload = make(map[string]interface{})
	if *notificationFileName != "" {
		data, err := ioutil.ReadFile(*notificationFileName)
		if err != nil {
			log.Fatalln("Error loading push file:", err)
		}
		err = json.Unmarshal(data, &payload)
		if err != nil {
			log.Fatalln("Error parsing push file:", err)
		}
	} else if *alert != "" {
		payload["aps"] = map[string]interface{}{
			"alert": *alert,
			"badge": *badge,
		}
	} else {
		log.Fatalln("Nothing to send")
	}
	cert, err := apns.LoadCertificate(*certFileName, *password)
	if err != nil {
		log.Fatalln("Error loading certificate:", err)
	}
	client := apns.New(*cert)
	if *development {
		client.Host = "https://api.development.push.apple.com"
	}
	for _, token := range tokens {
		id, err := client.Push(apns.Notification{
			Token:   token,
			Payload: payload,
			Topic:   *topic,
		})
		if err != nil {
			log.Println("Error:", err)
			break
		}
		log.Println("Sended:", id)
	}
	log.Println("Complete!")
}
