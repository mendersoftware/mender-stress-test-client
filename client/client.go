// Copyright 2023 Northern.tech AS
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

package client

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	mathrand "math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mendersoftware/go-lib-micro/ws"
	wsshell "github.com/mendersoftware/go-lib-micro/ws/shell"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/vmihailenco/msgpack/v5"

	"github.com/mendersoftware/mender-stress-test-client/model"
	"github.com/mendersoftware/mender-stress-test-client/websocket"
)

const urlAuthRequest = "/api/devices/v1/authentication/auth_requests"
const urlPutInventory = "/api/devices/v1/inventory/device/attributes"
const urlDeploymentsNext = "/api/devices/v1/deployments/device/deployments/next"
const urlDeploymentsStatus = "/api/devices/v1/deployments/device/deployments/{id}/status"

const websocketReconnectionIntervalInSeconds = 60

const (
	statusDownloading = "downloading"
	statusInstalling  = "installing"
	statusRebooting   = "rebooting"
	statusSuccess     = "success"
)

const (
	attributeRootfsImageVersion = "rootfs-image.version"
	attributeDeviceType         = "device_type"
)

var errUnauthorized = errors.New("unauthorized")

type Client struct {
	Index               int64
	MACAddress          string
	JWTToken            string
	Config              *model.RunConfig
	ArtifactName        string
	WebsocketConnection *websocket.Connection
}

type AuthRequest struct {
	IdentityData string `json:"id_data"`
	PublicKey    string `json:"pubkey"`
	TenantToken  string `json:"tenant_token"`
}

func getMACAddressFromPrefixAndIndex(prefix string, index int64) (string, error) {
	prefixNum, err := strconv.ParseUint(prefix, 16, 8)
	if err != nil {
		return "", err
	}
	buf := make([]byte, 6)
	buf[0] = byte(prefixNum)
	buf[1] = byte(int64(index>>32) & 255)
	buf[2] = byte(int64(index>>24) & 255)
	buf[3] = byte(int64(index>>16) & 255)
	buf[4] = byte(int64(index>>8) & 255)
	buf[5] = byte(int64(index) & 255)

	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", buf[0], buf[1], buf[2],
		buf[3], buf[4], buf[5]), nil
}

func NewClient(config *model.RunConfig, index int64) (*Client, error) {
	mathrand.Seed(time.Now().UnixNano() + index)
	macAddress, err := getMACAddressFromPrefixAndIndex(config.MACAddressPrefix, index)
	if err != nil {
		return nil, err
	}

	return &Client{
		Index:        index,
		MACAddress:   macAddress,
		Config:       config,
		ArtifactName: config.ArtifactName,
	}, nil
}

func (c *Client) Authenticate() error {
	identityData := map[string]string{"mac": c.MACAddress}
	for k, v := range c.Config.ExtraIdentity {
		identityData[k] = v
	}
	identityDataBytes, err := json.Marshal(identityData)
	if err != nil {
		return err
	}

	authRequest := &AuthRequest{
		IdentityData: string(identityDataBytes),
		PublicKey:    string(c.Config.PublicKey),
		TenantToken:  c.Config.TenantToken,
	}

	body, err := json.Marshal(authRequest)
	if err != nil {
		return err
	}

	hashed := sha256.Sum256(body)
	bodyHash, err := rsa.SignPKCS1v15(rand.Reader, c.Config.PrivateKey,
		crypto.SHA256, hashed[:])
	if err != nil {
		return err
	}
	signature := base64.StdEncoding.EncodeToString(bodyHash)

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true}

	for {
		buff := bytes.NewBuffer(body)
		req, err := http.NewRequest(http.MethodPost, c.Config.ServerURL+
			urlAuthRequest, buff)
		if err != nil {
			return err
		}
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("X-MEN-Signature", signature)

		start := time.Now()
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		elapsed := time.Since(start).Milliseconds()

		log.Debugf("[%s] %-40s %d (%6d ms)", c.MACAddress, "authentication",
			response.StatusCode, elapsed)

		if response.StatusCode == http.StatusOK {
			defer response.Body.Close()
			body, err := ioutil.ReadAll(response.Body)
			if err != nil {
				return err
			}
			c.JWTToken = string(body)
			return nil
		} else {
			response.Body.Close()
		}

		time.Sleep(c.Config.AuthInterval)
	}
}

func (c *Client) Run() {
	inventoryTicker := time.NewTicker(c.Config.InventoryInterval)
	updateTicker := time.NewTicker(c.Config.UpdateInterval)

auth:
	err := c.Authenticate()
	if err != nil {
		log.Errorf("[%s] %s", c.MACAddress, err)
		time.Sleep(c.Config.AuthInterval)
		goto auth
	}

	websocketMessages := make(chan *ws.ProtoMsg, 1)
	if c.Config.Websocket {
		go c.StartWebsocket(websocketMessages)
	}

	err = c.SendInventory()
	if err == errUnauthorized {
		goto auth
	}
	err = c.UpdateCheck()
	if err == errUnauthorized {
		goto auth
	}

	inventoryTicker.Reset(c.Config.InventoryInterval)
	updateTicker.Reset(c.Config.UpdateInterval)

	for {
		select {
		case <-inventoryTicker.C:
			err = c.SendInventory()
			if err == errUnauthorized {
				if c.Config.Websocket {
					_ = c.CloseWebsocket()
				}
				goto auth
			}
		case <-updateTicker.C:
			err = c.UpdateCheck()
			if err == errUnauthorized {
				if c.Config.Websocket {
					_ = c.CloseWebsocket()
				}
				goto auth
			}
		case msg := <-websocketMessages:
			log.Infof("[%s] websocket msg: %v", c.MACAddress, msg)
			if msg.Header.Proto == ws.ProtoTypeShell &&
				msg.Header.MsgType == wsshell.MessageTypeSpawnShell {
				_ = c.WebsocketConnection.WriteMessage(&ws.ProtoMsg{
					Header: ws.ProtoHdr{
						Proto:     msg.Header.Proto,
						MsgType:   msg.Header.MsgType,
						SessionID: msg.Header.SessionID,
						Properties: map[string]interface{}{
							"status": wsshell.ErrorMessage,
						},
					},
					Body: []byte("not supported by mender-stress-test-client"),
				})
			} else {
				b, _ := msgpack.Marshal(ws.Error{
					Error:        "handshake rejected",
					MessageProto: ws.ProtoTypeControl,
					MessageType:  ws.MessageTypeOpen,
					Close:        true,
				})
				_ = c.WebsocketConnection.WriteMessage(&ws.ProtoMsg{
					Header: ws.ProtoHdr{
						Proto:     ws.ProtoTypeControl,
						MsgType:   ws.MessageTypeError,
						SessionID: msg.Header.SessionID,
					},
					Body: b,
				})
			}
		}
	}
}

func (c *Client) SendInventory() error {
	attributes := []*model.InventoryAttribute{
		{
			Name:  attributeRootfsImageVersion,
			Value: c.ArtifactName,
		},
		{
			Name:  attributeDeviceType,
			Value: c.Config.DeviceType,
		},
	}
	for _, attr := range c.Config.InventoryAttributes {
		parts := strings.SplitN(attr, ":", 2)
		if len(parts) < 2 {
			continue
		}
		name := parts[0]
		values := strings.Split(parts[1], "|")
		value := values[int(c.Index)%len(values)]
		attributes = append(attributes, &model.InventoryAttribute{
			Name:  name,
			Value: value,
		})
	}

	for _, attr := range c.Config.InventoryAttributesRandom {
		parts := strings.SplitN(attr, ":", 2)
		if len(parts) < 2 {
			continue
		}
		name := parts[0]
		values := strings.Split(parts[1], "|")
		value := values[mathrand.Intn(len(values))]
		attributes = append(attributes, &model.InventoryAttribute{
			Name:  name,
			Value: value,
		})
	}

	body, err := json.Marshal(attributes)
	if err != nil {
		log.Errorf("[%s] %s", c.MACAddress, err)
		return err
	}

	req, err := http.NewRequest(http.MethodPut, c.Config.ServerURL+urlPutInventory,
		bytes.NewBuffer(body))
	if err != nil {
		log.Errorf("[%s] %s", c.MACAddress, err)
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+c.JWTToken)

	start := time.Now()
	response, err := http.DefaultClient.Do(req)
	if response != nil {
		response.Body.Close()
	}
	if err != nil {
		log.Errorf("[%s] %s", c.MACAddress, err)
		return err
	}
	elapsed := time.Since(start).Milliseconds()

	log.Debugf("[%s] %-40s %d (%6d ms)", c.MACAddress, "send-inventory",
		response.StatusCode, elapsed)
	if response.StatusCode == http.StatusUnauthorized {
		return errUnauthorized
	}

	return nil
}

func (c *Client) UpdateCheck() error {
	deploymentNextRequest := &model.DeploymentNextRequest{
		DeviceType:          c.Config.DeviceType,
		ArtifactName:        c.Config.ArtifactName,
		RootfsImageChecksum: c.Config.RootfsImageChecksum,
	}
	body, err := json.Marshal(deploymentNextRequest)
	if err != nil {
		log.Errorf("[%s] %s", c.MACAddress, err)
		return err
	}

	req, err := http.NewRequest(http.MethodPost, c.Config.ServerURL+urlDeploymentsNext,
		bytes.NewBuffer(body))
	if err != nil {
		log.Errorf("[%s] %s", c.MACAddress, err)
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+c.JWTToken)

	start := time.Now()
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Errorf("[%s] %s", c.MACAddress, err)
		return err
	}
	elapsed := time.Since(start).Milliseconds()
	defer response.Body.Close()

	log.Debugf("[%s] %-40s %d (%6d ms)", c.MACAddress, "update-check",
		response.StatusCode, elapsed)

	// unauthorized
	if response.StatusCode == http.StatusUnauthorized {
		return errUnauthorized
	}

	// received deployment
	if response.StatusCode == http.StatusOK {
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Errorf("[%s] %s", c.MACAddress, err)
			return err
		}

		response := &model.DeploymentNextResponse{}
		err = json.Unmarshal(body, response)
		if err != nil {
			log.Errorf("[%s] %s", c.MACAddress, err)
			return err
		}

		err = c.Deployment(response.ID)
		if err != nil {
			return err
		}

		// report the new artifact name
		if response.Artifact != nil {
			c.ArtifactName = response.Artifact.Name
		}
		err = c.SendInventory()
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) Deployment(deploymentID string) error {
	statusURL := strings.Replace(urlDeploymentsStatus, "{id}", deploymentID, 1)

	statuses := []string{
		statusDownloading,
		statusInstalling,
		statusRebooting,
		statusSuccess,
	}

	for _, status := range statuses {
		deploymentNextRequest := &model.DeploymentStatus{
			Status: status,
		}

		body, err := json.Marshal(deploymentNextRequest)
		if err != nil {
			log.Errorf("[%s] %s", c.MACAddress, err)
			return err
		}

		req, err := http.NewRequest(http.MethodPut, c.Config.ServerURL+statusURL,
			bytes.NewBuffer(body))
		if err != nil {
			log.Errorf("[%s] %s", c.MACAddress, err)
			return err
		}
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("Authorization", "Bearer "+c.JWTToken)

		start := time.Now()
		response, err := http.DefaultClient.Do(req)
		if response != nil {
			response.Body.Close()
		}
		if err != nil {
			log.Errorf("[%s] %s", c.MACAddress, err)
			return err
		}
		elapsed := time.Since(start).Milliseconds()

		log.Debugf("[%s] %-40s %d (%6d ms)", c.MACAddress, "deployment-status: "+status,
			response.StatusCode, elapsed)

		// unauthorized
		if response.StatusCode == http.StatusUnauthorized {
			return errUnauthorized
		}

		time.Sleep(c.Config.DeploymentTime)
	}
	return nil
}

func (c *Client) StartWebsocket(websocketMessages chan *ws.ProtoMsg) {
	interval := websocketReconnectionIntervalInSeconds * time.Second
	for {
		err := c.OpenWebsocket()
		if err != nil {
			log.Errorf("[%s] %s", c.MACAddress, err)
			time.Sleep(interval)
			continue
		}
		for {
			msg, err := c.WebsocketConnection.ReadMessage()
			if err != nil {
				_ = c.CloseWebsocket()
				time.Sleep(interval)
				break
			}
			websocketMessages <- msg
		}
	}
}

func (c *Client) OpenWebsocket() error {
	conn, err := websocket.NewConnection(c.Config.ServerURL, c.JWTToken)
	if err != nil {
		return err
	}
	log.Debugf("[%s] %-40s", c.MACAddress, "websocket connected")

	c.WebsocketConnection = conn
	return nil
}

func (c *Client) CloseWebsocket() error {
	log.Debugf("[%s] %-40s", c.MACAddress, "websocket disconnected")
	return c.WebsocketConnection.Close()
}
