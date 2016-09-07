package apns

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func TestCertificate(t *testing.T) {
	jsonEnc := json.NewEncoder(os.Stdout)
	jsonEnc.SetIndent("", "\t")
	for certfile, password := range map[string]string{
		"cert.p12":         "xopen123",
		"cert2.p12":        "xopen123",
		"cert3.p12":        "open321",
		"cert4.p12":        "xopen123",
		"TiTPushDev.p12":   "xopen123",
		"TiTUniversal.p12": "xopen123",
	} {
		fmt.Println("File:", certfile)
		certificate, err := LoadCertificate(certfile, password)
		if certificate == nil && err != nil {
			t.Error("Load certificate error:", err)
			continue
		} else if err != nil {
			fmt.Println("Certificate error:", err)
		}
		certificate.Leaf = nil
		info := GetCertificateInfo(*certificate)
		if info == nil {
			t.Error("Bad certificate info")
			continue
		}
		fmt.Println(info)
		if err := jsonEnc.Encode(info); err != nil {
			t.Error("JSON encode error:", err)
		}
		if !info.Support(info.BundleID) {
			t.Error("Unsupported bundle ID")
		}
		if info.Support("xxx") {
			t.Error("Bad support function")
		}
		fmt.Println(strings.Repeat("-", 60))
	}
}

func TestCertificateNotFound(t *testing.T) {
	if _, err := LoadCertificate("notexists.p12", "xopen"); err == nil {
		t.Error("Load not existing certificate error")
	}
	if _, err := LoadCertificate("cert.p12", "xopen"); err == nil {
		t.Error("Load not existing certificate error")
	}
}

func TestCertificateWithErrors(t *testing.T) {
	jsonEnc := json.NewEncoder(os.Stdout)
	jsonEnc.SetIndent("", "\t")
	for certfile, password := range map[string]string{
		"Certificates.p12":      "xopen123",
		"MessageTrack2Dev.p12":  "xopen123",
		"MessageTrack2Prod.p12": "xopen123",
		"MessageTrackDev.p12":   "xopen123",
		"MessageTrackProd.p12":  "xopen123",
		"MyMessagesDev.p12":     "xopen123",
		"MyMessagesProd.p12":    "xopen123",
	} {
		fmt.Println("File:", certfile)
		certificate, err := LoadCertificate(certfile, password)
		if certificate == nil && err != nil {
			fmt.Println("Load certificate error:", err)
			fmt.Println(strings.Repeat("-", 60))
			data, err := ioutil.ReadFile(certfile)
			if err != nil {
				t.Error(err)
				continue
			}
			certificate = &tls.Certificate{
				Certificate: [][]byte{data},
				PrivateKey:  nil,
				Leaf:        nil,
			}
			info := GetCertificateInfo(*certificate)
			if info != nil {
				t.Error("Bad info")
			}
			continue
		} else if err != nil {
			fmt.Println("Certificate error:", err)
		}
		certificate.Leaf = nil
		info := GetCertificateInfo(*certificate)
		if info == nil {
			t.Error("Bad certificate info")
			continue
		}
		fmt.Println(info)
		if err := jsonEnc.Encode(info); err != nil {
			t.Error("JSON encode error:", err)
		}
		if !info.Support(info.BundleID) {
			t.Error("Unsupported bundle ID")
		}
		if info.Support("xxx") {
			t.Error("Bad support function")
		}
		fmt.Println(strings.Repeat("-", 60))
	}
}
