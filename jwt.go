package apns

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"sync"
	"time"
)

// Error parsing token provider.
var (
	ErrPTBad           = errors.New("bad provider token")
	ErrPTBadKeyID      = errors.New("bad provider token key id")
	ErrPTBadTeamID     = errors.New("bad provider token team ID")
	ErrPTBadPrivateKey = errors.New("bad provider token private key")
)

// ProviderToken is Provider Authentication Tokens.
//
// If the provider token signing key is suspected to be compromised, you can
// revoke the key from your online developer account. You can issue a new key
// pair and generate new tokens with the new private key. For maximum security,
// it is recommended to close connections to APNs that were using the tokens
// signed with the revoked key and reconnect before using tokens signed with the
// new key.
type ProviderToken struct {
	teamID     [10]byte          // 10 character Team ID
	keyID      [10]byte          // 10 character Key ID
	privateKey *ecdsa.PrivateKey // private key for sign
	jwt        string            // cached JWT
	created    time.Time         // cache creation time
	mu         sync.RWMutex
}

// NewProviderToken returns a new ProviderToken with the established IDs team
// and key. Your Team ID and Key ID values can be obtained from your developer
// account.
func NewProviderToken(teamID, keyID string) (*ProviderToken, error) {
	jwt := new(ProviderToken)
	if len(teamID) != 10 {
		return nil, ErrPTBadTeamID
	}
	copy(jwt.teamID[:], teamID)
	if len(keyID) != 10 {
		return nil, ErrPTBadKeyID
	}
	copy(jwt.keyID[:], keyID)
	return jwt, nil
}

// LoadPrivateKey loads a private key from a file in PKCS8 format.
func (pt *ProviderToken) LoadPrivateKey(filename string) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	block, data := pem.Decode(data)
	if block != nil {
		data = block.Bytes
	}
	private, err := x509.ParsePKCS8PrivateKey(data)
	if err != nil {
		return err
	}
	privateKey, ok := private.(*ecdsa.PrivateKey)
	if !ok {
		return ErrPTBadPrivateKey
	}
	pt.mu.Lock()
	pt.jwt = ""
	pt.created = time.Time{}
	pt.privateKey = privateKey
	pt.mu.Unlock()
	return nil
}

// SetPrivateKey adds to the ProviderToken private key in the format of ASN.1.
func (pt *ProviderToken) SetPrivateKey(privateKey []byte) error {
	key, err := x509.ParseECPrivateKey(privateKey)
	if err != nil {
		return err
	}
	pt.mu.Lock()
	pt.jwt = ""
	pt.created = time.Time{}
	pt.privateKey = key
	pt.mu.Unlock()
	return nil
}

// String returns a string with the IDs team and key.
func (pt *ProviderToken) String() string {
	return fmt.Sprintf("%s:%s", pt.teamID, pt.keyID)
}

type jsonProviderToken struct {
	TeamID     string `json:"teamId"`
	KeyID      string `json:"keyId"`
	PrivateKey []byte `json:"privateKey"`
}

// MarshalJSON returns the description of the ProviderToken using the JSON
// format.
func (pt *ProviderToken) MarshalJSON() ([]byte, error) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	privateKey, err := x509.MarshalECPrivateKey(pt.privateKey)
	if err != nil {
		return nil, err
	}
	return json.Marshal(&jsonProviderToken{
		TeamID:     string(pt.teamID[:]),
		KeyID:      string(pt.keyID[:]),
		PrivateKey: privateKey,
	})
}

// UnmarshalJSON restores the ProviderToken from a JSON format.
func (pt *ProviderToken) UnmarshalJSON(data []byte) error {
	var jsonPT = new(jsonProviderToken)
	if err := json.Unmarshal(data, jsonPT); err != nil {
		return err
	}
	newPT, err := NewProviderToken(jsonPT.KeyID, jsonPT.TeamID)
	if err != nil {
		return err
	}
	*pt = *newPT
	return pt.SetPrivateKey(jsonPT.PrivateKey)
}

// JWTLifeTime contains the lifetime of the authorization token provider,
// through which it needs to be automatically updated.
//
// APNs will reject push messages with an Expired Provider Token error if the
// token issue timestamp is not within the last hour.
var JWTLifeTime = time.Minute * 55

// JWT returns a string with the signed authorization token in JWT format.
//
// The provider token that authorizes APNs to send push notifications for the
// specified topics. The token is in Base64URL-encoded JWT format, specified as
// bearer <provider token>.
//
// In order to ensure security, APNs requires new tokens to be generated
// periodically (confirm the validity). The new token will have an updated
// Issued At claim indicating the time when the token was generated. APNs will
// reject push messages with an Expired Provider Token error if the token issue
// timestamp is not within the last hour.
func (pt *ProviderToken) JWT() (string, error) {
	pt.mu.RLock()
	jwt := pt.jwt
	created := pt.created
	pt.mu.RUnlock()
	if jwt == "" || time.Since(created) > JWTLifeTime {
		return pt.createJWT()
	}
	return jwt, nil
}

// createJWT the JWT and store it in internal cache.
func (pt *ProviderToken) createJWT() (string, error) {
	if pt.privateKey == nil {
		return "", ErrPTBadPrivateKey
	}
	buf := []byte(`************` +
		`{"alg":"ES256","kid":"0000000000"}.` + // header
		`*************` +
		`{"iss":"0000000000","iat":0000000000}.` + // claims
		`*******************************************` +
		`*******************************************`) // sign
	// header
	copy(buf[34:44], pt.keyID[:10])
	base64.RawURLEncoding.Encode(buf[:46], buf[12:46])
	// claims
	copy(buf[68:78], pt.teamID[:10])
	created := time.Now()
	copy(buf[86:96], []byte(strconv.FormatInt(created.Unix(), 10)))
	base64.RawURLEncoding.Encode(buf[47:97], buf[60:97])
	// sign
	sum := sha256.Sum256(buf[:97])
	r, s, err := ecdsa.Sign(rand.Reader, pt.privateKey, sum[:])
	if err != nil {
		panic(err)
	}
	copy(buf[120:152], r.Bytes())
	copy(buf[152:186], s.Bytes())
	base64.RawURLEncoding.Encode(buf[98:186], buf[120:186])
	jwt := string(buf)
	pt.mu.Lock()
	pt.jwt = jwt
	pt.created = created
	pt.mu.Unlock()
	return jwt, nil
}

const providerTokenPEMType = "APNS TOKEN"

// WritePEM stores the ProviderToken in PEM format.
func (pt *ProviderToken) WritePEM(out io.Writer) error {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	privateKey, err := x509.MarshalECPrivateKey(pt.privateKey)
	if err != nil {
		return err
	}
	block := &pem.Block{
		Type: providerTokenPEMType,
		Headers: map[string]string{
			"teamID": string(pt.teamID[:]),
			"keyID":  string(pt.keyID[:]),
		},
		Bytes: privateKey,
	}
	return pem.Encode(out, block)
}

// ProviderTokenFromPEM parses and returns the description of the ProviderToken
// from PEM format.
func ProviderTokenFromPEM(data []byte) (*ProviderToken, error) {
	block, _ := pem.Decode(data)
	if block == nil || block.Type != providerTokenPEMType ||
		block.Headers == nil {
		return nil, ErrPTBad
	}
	pt, err := NewProviderToken(block.Headers["keyID"], block.Headers["teamID"])
	if err != nil {
		return nil, err
	}
	if err = pt.SetPrivateKey(block.Bytes); err != nil {
		return nil, err
	}
	return pt, nil
}
