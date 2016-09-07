package apns

import (
	"log"
	"sync"
	"testing"
)

func TestPool(t *testing.T) {
	tokens := []string{
		"BE311B5BADA725B323B1A56E03ED25B4814D6B9EDF5B02D3D605840860FEBB28", // iPad
		"507C1666D7ECA6C26F40BC322A35CCB937E2BF02DFDACA8FCCAAD5CEE580EE8C", // iPad mini
		"6B0420FA3B631DF5C13FB9DDC1BE8131C52B4E02580BB5F76BFA32862F284572", // iPhone
		"6B0420FA3B631DF5C13FB9DDC1BE8131C52B4E02580BB5F76BFA32862F284570", // Bad
	}
	var wg sync.WaitGroup
	wg.Add(len(tokens))
	responses := make(chan Response)
	go func() {
		for r := range responses {
			log.Println(r)
			wg.Done()
		}
	}()
	certificate, err := LoadCertificate("cert.p12", "xopen123")
	if err != nil {
		t.Fatal("Load certificate error:", err)
	}
	client := New(*certificate).Pool(2, responses)
	defer client.Close()
	n := Notification{Payload: `{"aps":{"alert":"Test message"}}`}
	client.Push(n, tokens...)
	wg.Wait()
}
