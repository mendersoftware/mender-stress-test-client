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
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	mrand "math/rand"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mendersoftware/log"
	"github.com/mendersoftware/mender/client"
	"github.com/mendersoftware/mender/datastore"
	"github.com/mendersoftware/mender/store"
)

var (
	menderClientCount        int
	maxWaitSteps             int
	inventoryUpdateFrequency int
	pollFrequency            int
	backendHost              string
	inventoryItems           string
	updateFailMsg            string
	updateFailCount          int
	currentArtifact          string
	currentDeviceType        string
	debugMode                bool
	singleKeyMode            bool
	substateReporting        bool
	startupInterval          int

	updatesPerformed  int
	updatesLeftToFail int

	tenantToken string

	lock sync.Mutex
)

type FakeMenderClient struct {
	mac string
	key string
}

type FakeMenderAuthManager struct {
	idSrc       []byte
	tenantToken string
	store       store.Store
	keyStore    *store.Keystore
}

func init() {
	flag.IntVar(&menderClientCount, "count", 100, "amount of fake mender clients to spawn")
	flag.IntVar(&maxWaitSteps, "wait", 1800, "max. amount of time to wait between update steps: download image, install, reboot, success/failure")
	flag.IntVar(&inventoryUpdateFrequency, "invfreq", 600, "amount of time to wait between inventory updates")
	flag.StringVar(&backendHost, "backend", "https://localhost", "entire URI to the backend")
	flag.StringVar(&inventoryItems, "inventory", "device_type:test,image_id:test,client_version:test", "inventory key:value pairs distinguished with ','")
	flag.StringVar(&updateFailMsg, "fail", strings.TrimSpace(strings.Repeat("failed, damn! ", 3)), "fail update with specified message")
	flag.IntVar(&updateFailCount, "failcount", 1, "amount of clients that will fail an update")

	flag.StringVar(&currentArtifact, "current_artifact", "test", "current installed artifact")
	flag.StringVar(&currentDeviceType, "current_device", "test", "current device type")

	flag.IntVar(&pollFrequency, "pollfreq", 600, "how often to poll the backend")
	flag.BoolVar(&debugMode, "debug", false, "debug output")
	flag.BoolVar(&singleKeyMode, "single_key", false, "single key mode: generates a single key and uses sequential mac addresses")

	flag.BoolVar(&substateReporting, "substate", false, "send substate reporting")
	flag.StringVar(&tenantToken, "tenant", "", "tenant key for account")

	flag.IntVar(&startupInterval, "startup_interval", 0, "Define the size (seconds) of the uniform interval on which the clients will start")

	mrand.Seed(time.Now().UnixNano())
}

func main() {
	flag.Parse()

	if len(os.Args) == 1 {
		flag.PrintDefaults()
		os.Exit(1)
	}

	if debugMode {
		log.SetLevel(log.DebugLevel)
	}

	updatesLeftToFail = updateFailCount

	if _, err := os.Stat("keys/"); os.IsNotExist(err) {
		err = os.Mkdir("keys", 0700)
		if err != nil {
			panic(err)
		}
	}

	clients := make([]*FakeMenderClient, menderClientCount)

	files, _ := filepath.Glob("keys/**")
	for i, filename := range files {
		clients[i] = &FakeMenderClient{
			mac: path.Base(filename),
			key: filename,
		}
	}

	keysMissing := menderClientCount - len(files)
	if keysMissing > 0 {
		fmt.Printf("%d keys need to be generated...\n", keysMissing)
		for keysMissing > 0 {
			index := menderClientCount - keysMissing
			mac, filename, err := generateClientKeys(index)
			if err != nil {
				log.Fatal("failed to generate crypto keys!")
			}
			clients[index] = &FakeMenderClient{
				mac: mac,
				key: filename,
			}
			keysMissing--
		}
	}

	delta := time.Duration(startupInterval / menderClientCount)
	for i := 0; i < menderClientCount; i++ {
		time.Sleep(delta * time.Second)
		go clientScheduler(clients[i])
	}

	// block forever
	select {}
}

func generateClientKeys(index int) (string, string, error) {
	buf := make([]byte, 6)
	if singleKeyMode {
		buf[0] = 255
		buf[1] = byte(int64(index>>32) & 255)
		buf[2] = byte(int64(index>>24) & 255)
		buf[3] = byte(int64(index>>16) & 255)
		buf[4] = byte(int64(index>>8) & 255)
		buf[5] = byte(int64(index) & 255)
	} else {
		if _, err := rand.Read(buf); err != nil {
			panic(err)
		}
	}

	fakeMACaddress := fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", buf[0], buf[1], buf[2], buf[3], buf[4], buf[5])
	log.Debug("created device with fake mac address: ", fakeMACaddress)

	if singleKeyMode && index == 0 || !singleKeyMode {
		ms := store.NewDirStore("keys/")
		kstore := store.NewKeystore(ms, fakeMACaddress, "", false, "")

		if err := kstore.Generate(); err != nil {
			return "", "", err
		}

		if err := kstore.Save(); err != nil {
			return "", "", err
		}
	} else {
		if err := os.Link("keys/ff:00:00:00:00:00", "keys/"+fakeMACaddress); err != nil {
			panic(err)
		}
	}

	return fakeMACaddress, "keys/" + fakeMACaddress, nil
}

func clientScheduler(menderClient *FakeMenderClient) {
	clientUpdateTicker := time.NewTicker(time.Second * time.Duration(pollFrequency))
	clientInventoryTicker := time.NewTicker(time.Second * time.Duration(inventoryUpdateFrequency))

	api, err := client.New(client.Config{
		IsHttps:  true,
		NoVerify: true,
	})

	if err != nil {
		log.Fatal(err)
	}

	storeFile := menderClient.key
	token := clientAuthenticate(api, storeFile)

	for {
		select {
		case <-clientInventoryTicker.C:
			invItems := parseInventoryItems()
			sendInventoryUpdate(api, token, &invItems)

		case <-clientUpdateTicker.C:
			checkForNewUpdate(api, token)
		}
	}
}

func clientAuthenticate(c *client.ApiClient, storeFile string) client.AuthToken {
	macAddress := filepath.Base(storeFile)
	identityData := map[string]string{"mac": macAddress}
	encdata, _ := json.Marshal(identityData)

	ms := store.NewDirStore(filepath.Dir(storeFile))
	kstore := store.NewKeystore(ms, macAddress, "", false, "")
	if err := kstore.Load(); err != nil {
		panic(err)
	}

	authReq := client.NewAuth()

	mgr := &FakeMenderAuthManager{
		store:       ms,
		keyStore:    kstore,
		idSrc:       encdata,
		tenantToken: tenantToken,
	}

	if err := kstore.Save(); err != nil {
		panic(err)
	}

	for {
		if authTokenResp, err := authReq.Request(c, backendHost, mgr); err == nil && len(authTokenResp) > 0 {
			return client.AuthToken(authTokenResp)
		} else if err != nil {
			log.Debug("not able to authorize client: ", err)
		}

		time.Sleep(time.Duration(pollFrequency) * time.Second)
	}
}

func stressTestClientServerIterator() func() *client.MenderServer {
	serverIteratorFlipper := true
	return func() *client.MenderServer {
		serverIteratorFlipper = !serverIteratorFlipper
		if serverIteratorFlipper {
			return nil
		}
		return &client.MenderServer{ServerURL: backendHost}
	}
}

func checkForNewUpdate(c *client.ApiClient, token client.AuthToken) {

	// if we performed an update for all the devices, we should reset the number of failed updates to perform
	if updatesPerformed > 0 && updatesPerformed%menderClientCount == 0 {
		updatesLeftToFail = updateFailCount
	}

	updater := client.NewUpdate()
	haveUpdate, err := updater.GetScheduledUpdate(c.Request(client.AuthToken(token),
		stressTestClientServerIterator(),
		func(string) (client.AuthToken, error) {
			return token, nil
		}), backendHost, &client.CurrentUpdate{DeviceType: currentDeviceType, Artifact: currentArtifact})

	if err != nil {
		log.Info("failed when checking for new updates with: ", err.Error())
	}

	if haveUpdate != nil {
		u := haveUpdate.(datastore.UpdateInfo)
		performFakeUpdate(u.Artifact.Source.URI, u.ID, c.Request(client.AuthToken(token),
			stressTestClientServerIterator(),
			func(string) (client.AuthToken, error) {
				return token, nil
			}))
	}
}

func performFakeUpdate(url string, did string, token client.ApiRequester) {
	s := client.NewStatus()
	substate := ""
	reportingCycle := []string{"downloading", "installing", "rebooting"}

	lock.Lock()
	if len(updateFailMsg) > 0 && updatesLeftToFail > 0 {
		reportingCycle = append(reportingCycle, "failure")
		updatesLeftToFail -= 1
	} else {
		reportingCycle = append(reportingCycle, "success")
	}
	updatesPerformed += 1
	lock.Unlock()

	for _, event := range reportingCycle {
		time.Sleep(15 + time.Duration(mrand.Intn(maxWaitSteps))*time.Second)
		if event == "downloading" {
			if err := downloadToDevNull(url); err != nil {
				log.Warn("failed to download update: ", err)
			}
		}

		if event == "failure" {
			logUploader := client.NewLog()

			ld := client.LogData{
				DeploymentID: did,
				Messages:     []byte(fmt.Sprintf("{\"messages\": [{\"level\": \"debug\", \"message\": \"%s\", \"timestamp\": \"2012-11-01T22:08:41+00:00\"}]}", updateFailMsg)),
			}

			if err := logUploader.Upload(token, backendHost, ld); err != nil {
				log.Warn("failed to deliver fail logs to backend: " + err.Error())
				return
			}
		}

		switch event {
		case "downloading":
			substate = "running predownload script"
		case "installing":
			substate = "running preinstalling script"
		case "rebooting":
			substate = "running prerebooting script"
		default:
			substate = ""
		}

		report := client.StatusReport{DeploymentID: did, Status: event, SubState: substate}
		err := s.Report(token, backendHost, report)

		if err != nil {
			log.Warn("error reporting update status: ", err.Error())
		}
	}
}

func sendInventoryUpdate(c *client.ApiClient, token client.AuthToken, invAttrs *[]client.InventoryAttribute) {
	log.Debug("submitting inventory update with: ", invAttrs)
	if err := client.NewInventory().Submit(c.Request(client.AuthToken(token),
		stressTestClientServerIterator(),
		func(string) (client.AuthToken, error) {
			return token, nil
		}),
		backendHost, invAttrs); err != nil {
		log.Warn("failed sending inventory with: ", err.Error())
	}
}

func downloadToDevNull(url string) error {
	log.Info("downloading url")
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	resp, err := client.Get(url)
	if err != nil {
		log.Error("failed grabbing update: ", url)
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(ioutil.Discard, resp.Body)

	if err != nil {
		return err
	}
	log.Debug("downloaded update successfully to /dev/null")
	return nil
}

func parseInventoryItems() []client.InventoryAttribute {
	var invAttrs []client.InventoryAttribute
	for _, e := range strings.Split(inventoryItems, ",") {
		pair := strings.Split(e, ":")
		if pair != nil {
			key := pair[0]
			value := pair[1]
			i := client.InventoryAttribute{Name: key, Value: value}
			invAttrs = append(invAttrs, i)
		}
	}
	// add a dynamic inventory inventoryItems
	i := client.InventoryAttribute{Name: "time", Value: time.Now().Unix()}
	invAttrs = append(invAttrs, i)
	return invAttrs
}
