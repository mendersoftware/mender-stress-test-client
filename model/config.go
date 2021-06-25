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

package model

import (
	"crypto/rsa"
	"time"
)

type RunConfig struct {
	Count               int64
	KeyFile             string
	MACAddressPrefix    string
	DeviceType          string
	ArtifactName        string
	RootfsImageChecksum string
	InventoryAttributes []string
	StartTime           time.Duration
	AuthInterval        time.Duration
	InventoryInterval   time.Duration
	UpdateInterval      time.Duration
	DeploymentTime      time.Duration
	ServerURL           string
	TenantToken         string
	PrivateKey          *rsa.PrivateKey
	PublicKey           []byte
	Websocket           bool
}
