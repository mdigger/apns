package apns

import (
	"github.com/kr/pretty"
	"testing"
)

func TestFeedback(t *testing.T) {
	config, err := LoadConfig("config.json")
	if err != nil {
		t.Fatal(err)
	}
	response, err := config.Feedback()
	if err != nil {
		t.Fatal(err)
	}
	pretty.Print(response)
}
