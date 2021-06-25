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
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"github.com/mendersoftware/mender-stress-test-client/model"
)

func main() {
	doMain(os.Args)
}

func doMain(args []string) {
	app := &cli.App{
		Commands: []cli.Command{
			{
				Name:   "run",
				Usage:  "Run the clients",
				Action: cmdRun,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "server-url",
						Usage: "Server's URL",
						Value: "https://localhost",
					},
					&cli.StringFlag{
						Name:  "tenant-token",
						Usage: "Tenant token",
					},
					&cli.IntFlag{
						Name:  "count",
						Usage: "Number of clients to run",
						Value: 100,
					},
					&cli.IntFlag{
						Name: "start-time",
						Usage: "Start up time in seconds; the clients " +
							"will spwan uniformly in the given " +
							"amount of time",
						Value: 10,
					},
					&cli.StringFlag{
						Name:  "key-file",
						Usage: "Path to the key file to use",
						Value: "private.key",
					},
					&cli.StringFlag{
						Name: "mac-address-prefix",
						Usage: "MAC addresses first byte prefix, in hex " +
							"format",
						Value: "ff",
					},
					&cli.StringFlag{
						Name:  "device-type",
						Usage: "Device type",
						Value: "test",
					},
					&cli.StringFlag{
						Name:  "rootfs-image-checksum",
						Usage: "Checksum of the rootfs image",
						Value: "4d480539cdb23a4aee6330ff80673a5a" +
							"f92b7793eb1c57c4694532f96383b619",
					},
					&cli.StringFlag{
						Name:  "artifact-name",
						Usage: "Name of the current installed artifact",
						Value: "original",
					},
					&cli.StringSliceFlag{
						Name: "inventory-attribute",
						Usage: "Inventory attribute, in the form of " +
							"key:value1|value2",
						Value: &cli.StringSlice{
							"device_type:test",
							"image_id:test",
							"client_version:test",
							"device_group:group1|group2",
						},
					},
					&cli.IntFlag{
						Name:  "auth-interval",
						Usage: "auth interval in seconds",
						Value: 600,
					},
					&cli.IntFlag{
						Name:  "inventory-interval",
						Usage: "Inventory poll interval in seconds",
						Value: 1800,
					},
					&cli.IntFlag{
						Name:  "update-interval",
						Usage: "Update poll interval in seconds",
						Value: 600,
					},
					&cli.IntFlag{
						Name: "deployment-time",
						Usage: "Wait time between deployment steps " +
							"(downloading, installing, rebooting, " +
							"success)",
						Value: 30,
					},
					&cli.BoolFlag{
						Name:  "websocket",
						Usage: "Enable websocket mode",
					},
					&cli.BoolFlag{
						Name:  "debug",
						Usage: "Enable debug mode",
					},
				},
			},
		},
	}

	err := app.Run(args)
	if err != nil {
		log.Fatal(err)
	}
}

func cmdRun(args *cli.Context) error {
	if args.Bool("debug") {
		log.SetLevel(log.DebugLevel)
	}

	config := &model.RunConfig{
		Count:               args.Int64("count"),
		KeyFile:             args.String("key-file"),
		StartTime:           time.Duration(args.Int("start-time")) * time.Second,
		MACAddressPrefix:    args.String("mac-address-prefix"),
		ArtifactName:        args.String("artifact-name"),
		DeviceType:          args.String("device-type"),
		RootfsImageChecksum: args.String("rootfs-image-checksum"),
		InventoryAttributes: args.StringSlice("inventory-attribute"),
		AuthInterval:        time.Duration(args.Int("auth-interval")) * time.Second,
		InventoryInterval:   time.Duration(args.Int("inventory-interval")) * time.Second,
		UpdateInterval:      time.Duration(args.Int("update-interval")) * time.Second,
		DeploymentTime:      time.Duration(args.Int("deployment-time")) * time.Second,
		ServerURL:           args.String("server-url"),
		TenantToken:         args.String("tenant-token"),
		Websocket:           args.Bool("websocket"),
	}
	return run(config)
}
