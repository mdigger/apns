package apns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestClient(t *testing.T) {
	certificate, err := LoadCertificate("cert.p12", "xopen123")
	if err != nil {
		t.Fatal("Load certificate error:", err)
	}
	client := New(*certificate)
	for _, token := range []string{
		"BE311B5BADA725B323B1A56E03ED25B4814D6B9EDF5B02D3D605840860FEBB28", // iPad
		"507C1666D7ECA6C26F40BC322A35CCB937E2BF02DFDACA8FCCAAD5CEE580EE8C", // iPad mini
		"6B0420FA3B631DF5C13FB9DDC1BE8131C52B4E02580BB5F76BFA32862F284572", // iPhone
		// "6B0420FA3B631DF5C13FB9DDC1BE8131C52B4E02580BB5F76BFA32862F284570", // Bad
	} {
		id, err := client.Push(Notification{
			Token:   token,
			Payload: `{"aps":{"alert":"Test message"}}`,
		})
		fmt.Println(id)
		if err != nil {
			t.Error("Push error:", err)
		}
	}
}

func TestClient2(t *testing.T) {
	certificate, err := LoadCertificate("cert.p12", "xopen123")
	if err != nil {
		t.Fatal("Load certificate error:", err)
	}
	client := New(*certificate)

	_, err = client.Push(Notification{
		Payload: []byte(`{"aps":{"alert":"Test message"}}`),
		Token:   "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX",
	})
	if err == nil {
		t.Error("bad token format")
	}
	_, err = client.Push(Notification{
		Payload: json.RawMessage(bytes.Repeat([]byte("x"), 5000)),
	})
	if err == nil {
		t.Error("bad payload size checking")
	}
	_, err = client.Push(Notification{
		Payload: complex(float64(128), float64(6)),
	})
	if err == nil {
		t.Error("bad payload format")
	}
	_, err = client.Push(Notification{
		Payload: map[string]interface{}{
			"test":   "message",
			"number": 2,
		},
		Token: "XXXXXXXX",
	})
	if err == nil {
		t.Error("bad token size")
	}
	client.Sandbox = true
	_, err = client.Push(Notification{
		ID:          "123e4567-e89b-12d3-a456-42665544000",
		Expiration:  time.Now().Add(time.Hour),
		LowPriority: true,
		Token:       "BE311B5BADA725B323B1A56E03ED25B4814D6B9EDF5B02D3D605840860FEBB28",
		Payload:     `{"aps":{"alert":"Test message"}}`,
	})
	if err == nil {
		t.Error("unregistered token for topic support")
	}
}

func TestClientGoroutine(t *testing.T) {
	certificate, err := LoadCertificate("cert.p12", "xopen123")
	if err != nil {
		t.Fatal("Load certificate error:", err)
	}
	client := New(*certificate)
	tokens := []string{
		"BE311B5BADA725B323B1A56E03ED25B4814D6B9EDF5B02D3D605840860FEBB28", // iPad
		"507C1666D7ECA6C26F40BC322A35CCB937E2BF02DFDACA8FCCAAD5CEE580EE8C", // iPad mini
		"6B0420FA3B631DF5C13FB9DDC1BE8131C52B4E02580BB5F76BFA32862F284572", // iPhone
		// "6B0420FA3B631DF5C13FB9DDC1BE8131C52B4E02580BB5F76BFA32862F284570", // Bad
	}
	var wg sync.WaitGroup
	wg.Add(len(tokens))
	for _, token := range tokens {
		go func(token string) {
			id, err := client.Push(Notification{
				Token:   token,
				Payload: `{"aps":{"alert":"Test message"}}`,
			})
			fmt.Println(id)
			wg.Done()
			if err != nil {
				fmt.Println("Push error:", err)
			}
		}(token)
	}
	wg.Wait()
}
