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

package main

import (
	"time"

	"github.com/mendersoftware/mender-stress-test-client/client"
	"github.com/mendersoftware/mender-stress-test-client/key"
	"github.com/mendersoftware/mender-stress-test-client/model"
)

func run(config *model.RunConfig) error {
	key, publicKey, err := key.GetPublicPrivateKey(config)
	if err != nil {
		return err
	}

	config.PrivateKey = key
	config.PublicKey = publicKey

	for i := int64(0); i < config.Count; i++ {
		client, err := client.NewClient(config, i)
		if err != nil {
			return err
		}
		go client.Run()

		time.Sleep(config.StartTime / time.Duration(config.Count))
	}

	select {}
}
