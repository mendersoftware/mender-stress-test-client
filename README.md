# mender-stress-test-client

[![Build Status](https://gitlab.com/Northern.tech/Mender/mender-stress-test-client/badges/master/pipeline.svg)](https://gitlab.com/Northern.tech/Mender/mender-stress-test-client/pipelines)

The `mender-stress-test-client` is a dummy-client intended for simple testing of
the Mender server functionality or scale testing. It is not in any way a
fully-featured client. All it does is mimic a client's API and doesn't download the
artifacts when performing an update. This means that it has **no state** whatsoever,
all updates are instantly thrown away, and the client will forever function as it
did once booted up. This means that all parameters provided to the client at startup
will stay this way until the client is brought down.

## Getting Started

### Building

The `mender-stress-test-client` can be built by simply running `go build .` in
the mender-stress-test-client repository. This will put an executable binary in
the current repo. If a universal option is wanted, the binary can be automatically
install into the `~/$GOPATH/bin` repository by running `go install`, and if this
is already in your `$PATH` variable, then the `mender-stress-test-client` will
function exactly like any other binary in your `PATH`.

### Running

If the client is already installed, run it with the default options like so:
`mender-stress-test-client run --count=<device-count>`

Pass the `-h` flag for all options.

```
./mender-stress-test-client run --help
NAME:
    run - Run the clients

USAGE:
    run [command options] [arguments...]

OPTIONS:
   --server-url value             Server's URL (default: "https://localhost")
   --tenant-token value           Tenant token
   --count value                  Number of clients to run (default: 100)
   --start-time value             Start up time in seconds; the clients will spwan uniformly in the given amount of time (default: 10)
   --key-file value               Path to the key file to use (default: "private.key")
   --mac-address-prefix value     MAC addresses first byte prefix, in hex format (default: "ff")
   --device-type value            Device type (default: "test")
   --rootfs-image-checksum value  Checksum of the rootfs image (default: "4d480539cdb23a4aee6330ff80673a5af92b7793eb1c57c4694532f96383b619")
   --artifact-name value          Name of the current installed artifact (default: "original")
   --inventory-attribute value    Inventory attribute, in the form of key:value1|value2 (default: "device_type:test", "image_id:test", "client_version:test", "device_group:group1|group2")
   --auth-interval value          auth interval in seconds (default: 600)
   --inventory-interval value     Inventory poll interval in seconds (default: 1800)
   --update-interval value        Update poll interval in seconds (default: 600)
   --deployment-time value        Wait time between deployment steps (downloading, installing, rebooting, success) (default: 30)
   --debug                        Enable debug mode
  ```
  
## Working with the Client

NOTE: Currently, there are some oddities to be had from the client, most notably:

* It has CLI options for inventory attributes **and** for device type and the currently
installed artifact; even though the latter two are generally part of the inventory,
these two are separate in the stress-test-client. Meaning that the inventory set
from the CLI, will always be the one that shows up on the server, regardless of
what is set as the current device type and the current artifact name. The
command-line options of the current device and current artifact are used to
build the _update request to the server_, and hence specify dummy names,
so that one can mismatch on the Artifact name, and match on the client type. In
general, these flags should match up, though, and hence be the same. If not
specified, these will default to the device type *test* and the artifact name *original*.

* As mentioned above, in general, the client has **no** state. However, this is
not wholly true, as the client will generate a private key and use it on subsequent
startups. If the client has no key to start with, it will generate one and store
it as `private.key`. Thus on killing the clients and starting them back up, the key
will still be present in the directory, and the clients will start back up with
the same key it had on the previous run.

* As mentioned, the client has no state! This implies that it can be 'updated'
with an artifact, but it will keep track of the current installed artifact in memory.
When restarted, the clients will report the original artifact name as specified
by the `--artifact-name` option again. The update is always discarded on the client
side (it is not downloaded nor parsed by mender-artifact).

* It reports the update phases, _downloading_, _installing_, _rebooting_ and
_success_. The client's time between each of these stages is determined
by the CLI-parameter `--deployment-time=<max-wait>`, and defaults to `30`.


## Working with the Demo Server

Following are the considerations needed when running the client with the Mender
Demo server. To run the Demo server with *N* clients, (this assumes the demo server
is already running), there are a few options that can be worthwhile consideration.

* First, the number of clients is specified with the `--count=<nr-devices>` flag.
  If this is a relatively large number, consider using the `--start-time=<last-start>`
  flag, to not bombard the server with N authorization calls at once. This can be
  used to bring the clients up at an interval from `[time-now,time-now + last-start]`.
  Hence, significantly lightening the upstart load on the server.

* The address of the Demo server is assumed by default to be on `localhost` but
  can be overridden by the `--server-url=<server-URL>` flag.
