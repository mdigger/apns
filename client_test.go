package apns

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"
)

var tokenStrings = []string{
	"F389410AE1B57972DBBF6EB0C05C2626AB69EDE88F523D7EED49FA6E63A6C266",
	"B8108B88198789E9696E11A2FFE9710B776A9851673C2FDEDFCE1BE318AE7C90",
}

func TestClient(t *testing.T) {
	config, err := LoadConfig("config.json")
	if err != nil {
		t.Fatal(err)
	}
	var tokens = make([][]byte, len(tokenStrings))
	for i, str := range tokenStrings {
		token, err := hex.DecodeString(str)
		if err != nil {
			t.Fatal(err)
		}
		tokens[i] = token
	}
	client := NewClient(config)
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// client.Delay = time.Duration(0)

	var wg sync.WaitGroup
	total := 5000
	streams := 1
	wg.Add(total / streams * streams)
	start := time.Now()
	for y := 0; y < streams; y++ {
		func(y int) {
			for i := 0; i < total/streams; i++ {
				ntf := &Notification{Payload: map[string]interface{}{
					"aps": map[string]interface{}{
						"alert": fmt.Sprintf("Test message %d-%d", y+1, i+1),
						"badge": i,
					},
					"time":   time.Now().Format(time.RFC3339Nano),
					"uint32": rand.Uint32(),
					"inf64":  rand.Int63(),
					"float":  rand.Float64(),
				}}
				client.Send(ntf, tokens...)
				wg.Done()
				// time.Sleep(50 * time.Millisecond)
				// if i%(rand.Intn(59)+1) == 0 {
				// 	time.Sleep(time.Duration(rand.Intn(70)) * time.Millisecond)
				// }
			}
		}(y)
	}
	wg.Wait()                        // ждем, пока не будут отправлены все сообщения
	for client.queue.IsHasToSend() { // ждем, пока очередь не пуста
		time.Sleep(time.Millisecond * 100)
	}
	fmt.Println("Complete! Time:", time.Since(start).String())
}
