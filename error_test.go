package apns

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestErrors(t *testing.T) {
	var timestamp = strconv.FormatInt(time.Now().Unix()*1000, 10)
	for status, body := range map[int]string{
		0:   `{"reason": "Test"}`,
		400: `{"reason": "PayloadEmpty"}`,
		410: `{"reason": "Unregistered", "timestamp": ` + timestamp + `}`,
		401: `{"reason": "BadDeviceToken"}`,
		500: `{xxxx}`,
	} {
		err := decodeError(status, strings.NewReader(body))
		if apnsErr, ok := err.(*Error); ok {
			fmt.Println(apnsErr.IsToken(), apnsErr.Time(), apnsErr)
		} else {
			fmt.Println("Error:", err)
		}
	}
}
