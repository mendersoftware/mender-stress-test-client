// Copyright 2021 Northern.tech AS
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//        http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package key

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"os"

	"github.com/mendersoftware/mender-stress-test-client/model"
	log "github.com/sirupsen/logrus"
)

const keyBitsSize = 3072

func GetPublicPrivateKey(config *model.RunConfig) (*rsa.PrivateKey, []byte, error) {
	var key *rsa.PrivateKey
	if _, err := os.Stat(config.KeyFile); os.IsNotExist(err) {
		log.Debug("key file doesn't exist, generating it")
		key, err = generatePrivateKey(keyBitsSize)
		if err != nil {
			return nil, nil, err
		}
		data := encodePrivateKeyToPEM(key)
		err = ioutil.WriteFile(config.KeyFile, data, 0600)
		if err != nil {
			return nil, nil, err
		}
	} else {
		key, err = readPrivateKeyFromPEM(config.KeyFile)
		if err != nil {
			return nil, nil, err
		}
	}

	publicKey, err := encodePublicKeyToPEM(&key.PublicKey)
	if err != nil {
		return nil, nil, err
	}

	return key, publicKey, nil
}

func generatePrivateKey(bitSize int) (*rsa.PrivateKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		return nil, err
	}
	err = privateKey.Validate()
	if err != nil {
		return nil, err
	}
	log.Info("private key generated")
	return privateKey, nil
}

func encodePublicKeyToPEM(publicKey *rsa.PublicKey) ([]byte, error) {
	pubDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, err
	}
	pubBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER,
	}
	pubPEM := pem.EncodeToMemory(pubBlock)
	return pubPEM, nil
}

func encodePrivateKeyToPEM(privateKey *rsa.PrivateKey) []byte {
	privDER := x509.MarshalPKCS1PrivateKey(privateKey)
	privBlock := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privDER,
	}
	privatePEM := pem.EncodeToMemory(&privBlock)
	return privatePEM
}

func readPrivateKeyFromPEM(filename string) (*rsa.PrivateKey, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(data)
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}
