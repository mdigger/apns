// Создает новый конфигурационный файл из указанных файлов с сертификатами.
//
// Если bundle id не указан, то приложение пытается найти его описание непосредственно в файле
// с сертификатом. Но гарантировать этого не возможно, поэтому всегда внимательно проверяйте,
// что bundle id указан корректно.
//
// Так же стоит обратить внимание, что файл с сертификатами не должен требовать пароля. Если это
// не так, то приложение не сможет его прочитать.
package main

import (
	"crypto/tls"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"github.com/mdigger/apns"
)

func main() {
	var (
		certFile   = flag.String("cert", "cert.pem", "certificate file name")
		keyFile    = flag.String("key", "key.pem", "private key file name")
		sandbox    = flag.Bool("sandbox", true, "sandbox mode")
		bundleId   = flag.String("bundle", os.Getenv("BUNDLE_ID"), "bundle id (if empty trying to find in certificate file info)")
		outputFile = flag.String("output", "config.json", "output filename")
	)
	flag.Parse()

	config, err := CreateConfig(*bundleId, *certFile, *keyFile, *sandbox)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	data, err := json.MarshalIndent(config, "", "\t")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	if err = ioutil.WriteFile(*outputFile, data, 0600); err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Created:", *outputFile)
}

// CreateConfig создает описание конфигурации, загружая сертификаты из указанных файлов.
func CreateConfig(bundleId, certFile, keyFile string, sandbox bool) (*apns.ConfigJSON, error) {
	certPEMBlock, err := ioutil.ReadFile(certFile)
	if err != nil {
		return nil, err
	}
	if bundleId == "" {
		bundlerIds := regexp.MustCompile(`subject=\/UID=([\w\.\-]{3,})\/`).
			FindStringSubmatch(string(certPEMBlock))
		if len(bundlerIds) > 1 {
			bundleId = bundlerIds[1]
		}
	}

	keyPEMBlock, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}
	if _, err = tls.X509KeyPair(certPEMBlock, keyPEMBlock); err != nil {
		return nil, err
	}

	var (
		cert         = make([][]byte, 0, 2)
		certDERBlock *pem.Block
	)
	for {
		certDERBlock, certPEMBlock = pem.Decode(certPEMBlock)
		if certDERBlock == nil {
			break
		}
		if certDERBlock.Type == "CERTIFICATE" {
			cert = append(cert, pem.EncodeToMemory(certDERBlock))
		}
	}
	if len(cert) == 0 {
		return nil, errors.New("No certificates find")
	}

	var keyDERBlock *pem.Block
	for {
		keyDERBlock, keyPEMBlock = pem.Decode(keyPEMBlock)
		if keyDERBlock == nil {
			return nil, errors.New("failed to parse key PEM data")
		}
		if keyDERBlock.Type == "PRIVATE KEY" ||
			strings.HasSuffix(keyDERBlock.Type, " PRIVATE KEY") {
			break
		}
	}

	var config = &apns.ConfigJSON{
		Type:        "apns",
		BundleID:    bundleId,
		Sandbox:     sandbox,
		Certificate: cert,
		PrivateKey:  pem.EncodeToMemory(keyDERBlock),
	}
	return config, nil
}
