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
	"encoding/json"
	"flag"
	"fmt"
	"github.com/mdigger/apns2"
	"io/ioutil"
)

func main() {
	certFile := flag.String("cert", "cert.pem", "certificate file name")
	keyFile := flag.String("key", "key.pem", "private key file name")
	sandbox := flag.Bool("sandbox", true, "sandbox mode")
	bundleId := flag.String("bundle", "", "bundle id (if empty trying to find in certificate file info)")
	outputFile := flag.String("output", "config.json", "output filename")
	flag.Parse()

	config, err := apns.CreateConfig(*bundleId, *certFile, *keyFile, *sandbox)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	data, err := json.MarshalIndent(config, "", "\t")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	if err := ioutil.WriteFile(*outputFile, data, 0600); err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Created:", *outputFile)
}
