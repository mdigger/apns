package apns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/kr/pretty"
)

func TestJWT(t *testing.T) {
	teamID, keyID, filename := "W23G28NPJW", "67XV3VSJ95", "APNSAuthKey_67XV3VSJ95.p8"
	pt, err := NewProviderToken(teamID, keyID)
	if err != nil {
		t.Fatal(err)
	}
	err = pt.LoadPrivateKey(filename)
	if err != nil {
		t.Fatal(err)
	}
	if str := pt.String(); str != fmt.Sprintf("%s:%s", teamID, keyID) {
		t.Error("bad ProviderToken string:", str)
	}

	var buf bytes.Buffer
	err = pt.WritePEM(&buf)
	if err != nil {
		t.Fatal(err)
	}
	newPT, err := ProviderTokenFromPEM(buf.Bytes())
	if err != nil {
		t.Error(err)
	}
	// buf.WriteTo(os.Stdout)

	buf.Reset()
	err = json.NewEncoder(&buf).Encode(pt)
	if err != nil {
		t.Fatal(err)
	}
	err = json.NewDecoder(&buf).Decode(newPT)
	if err != nil {
		t.Error(err)
	}
	// enc := json.NewEncoder(os.Stdout)
	// enc.SetIndent("", "  ")
	// enc.Encode(pt)
	fmt.Println(pt.JWT())
}

func TestVerifyJWT(t *testing.T) {
	teamID, keyID, filename := "W23G28NPJW", "67XV3VSJ95", "APNSAuthKey_67XV3VSJ95.p8"
	pt, err := NewProviderToken(teamID, keyID)
	if err != nil {
		t.Fatal(err)
	}
	err = pt.LoadPrivateKey(filename)
	if err != nil {
		t.Fatal(err)
	}
	tokenString := pt.JWT()

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return pt.privateKey.Public(), nil
	})
	if err != nil {
		pretty.Println(err)
	}
	if !token.Valid {
		pretty.Println(token)
	}
}

func TestClientJWT(t *testing.T) {
	teamID, keyID, filename := "W23G28NPJW", "67XV3VSJ95", "APNSAuthKey_67XV3VSJ95.p8"
	pt, err := NewProviderToken(teamID, keyID)
	if err != nil {
		t.Fatal(err)
	}
	err = pt.LoadPrivateKey(filename)
	if err != nil {
		t.Fatal(err)
	}

	client := NewWithToken(pt)
	for _, token := range []string{
		"BE311B5BADA725B323B1A56E03ED25B4814D6B9EDF5B02D3D605840860FEBB28", // iPad
		"507C1666D7ECA6C26F40BC322A35CCB937E2BF02DFDACA8FCCAAD5CEE580EE8C", // iPad mini
		"6B0420FA3B631DF5C13FB9DDC1BE8131C52B4E02580BB5F76BFA32862F284572", // iPhone
	} {
		id, err := client.Push(Notification{
			Token:   token,
			Topic:   "com.xyzrd.trackintouch",
			Payload: `{"aps":{"alert":"JWT Client test message"}}`,
		})
		fmt.Println(id)
		if err != nil {
			t.Error("Push error:", err)
		}
	}
}
