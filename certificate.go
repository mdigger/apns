package apns

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/asn1"
	"io/ioutil"
	"time"

	"golang.org/x/crypto/pkcs12"
)

// LoadCertificate return parsed TLS certificate from .p12 file.
func LoadCertificate(filename, password string) (*tls.Certificate, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	privateKey, x509Cert, err := pkcs12.Decode(data, password)
	if err != nil {
		return nil, err
	}
	cert := &tls.Certificate{
		Certificate: [][]byte{x509Cert.Raw},
		PrivateKey:  privateKey,
		Leaf:        x509Cert,
	}
	if _, err = x509Cert.Verify(x509.VerifyOptions{}); err != nil {
		if _, ok := err.(x509.UnknownAuthorityError); !ok {
			return cert, err
		}
	}
	return cert, nil
}

// CertificateInfo describes information about the certificate.
type CertificateInfo struct {
	CName       string    // certificate full name
	OrgName     string    // organization name
	OrgUnit     string    // organization identifier
	Country     string    // country
	BundleID    string    // bundle ID
	Topics      []string  // supported topics
	Development bool      // sandbox support flag
	Production  bool      // production support flag
	IsApple     bool      // certificate signed by Apple flag
	Expire      time.Time // expire date and time
}

// GetCertificateInfo parses and returns information about the certificate.
func GetCertificateInfo(certificate tls.Certificate) *CertificateInfo {
	var cert = certificate.Leaf
	if cert == nil {
		var err error
		cert, err = x509.ParseCertificate(certificate.Certificate[0])
		if err != nil {
			return nil
		}
	}
	var info = &CertificateInfo{
		CName:   cert.Subject.CommonName,
		Expire:  cert.NotAfter,
		IsApple: cert.Issuer.CommonName == appleDevIssuerCN,
	}
	for _, attr := range cert.Subject.Names {
		switch t := attr.Type; {
		case t.Equal(typeOrgName):
			info.OrgName = attr.Value.(string)
		case t.Equal(typeOrgUnit):
			info.OrgUnit = attr.Value.(string)
		case t.Equal(typeBundle):
			info.BundleID = attr.Value.(string)
		case t.Equal(typeCountry):
			info.Country = attr.Value.(string)
		}
	}
	for _, attr := range cert.Extensions {
		switch t := attr.Id; {
		case t.Equal(typeDevelopmet): // Development
			info.Development = true
		case t.Equal(typeProduction): // Production
			info.Production = true
		case t.Equal(typeTopics): // Topics
			var raw asn1.RawValue // разбираем корневой элемент списка
			if _, err := asn1.Unmarshal(attr.Value, &raw); err != nil {
				continue
			}
			info.Topics = make([]string, 0)
			for rest := raw.Bytes; len(rest) > 0; {
				var err error
				var topic string
				if rest, err = asn1.Unmarshal(rest, &topic); err != nil {
					break
				}
				info.Topics = append(info.Topics, topic)
				var names []string
				if rest, err = asn1.Unmarshal(rest, &names); err != nil {
					break
				}
			}
			// check for topics support bundle ID
			if !info.Support(info.BundleID) {
				panic("topics not support bundle ID")
			}
		}
	}
	return info
}

// Support returns true, if the certificate support the specified topic.
func (i CertificateInfo) Support(topic string) bool {
	if len(i.Topics) == 0 {
		return (topic == i.BundleID)
	}
	for _, name := range i.Topics {
		if name == topic {
			return true
		}
	}
	return false
}

// String return certificate CName.
func (i CertificateInfo) String() string {
	return i.CName
}

const appleDevIssuerCN = "Apple Worldwide Developer Relations Certification Authority"

var (
	typeCountry    = asn1.ObjectIdentifier{2, 5, 4, 6}
	typeOrgName    = asn1.ObjectIdentifier{2, 5, 4, 10}
	typeOrgUnit    = asn1.ObjectIdentifier{2, 5, 4, 11}
	typeBundle     = asn1.ObjectIdentifier{0, 9, 2342, 19200300, 100, 1, 1}
	typeDevelopmet = asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 6, 3, 1}
	typeProduction = asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 6, 3, 2}
	typeTopics     = asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 6, 3, 6}
)
