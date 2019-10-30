# mender-stress-test-client

[![Build Status](https://gitlab.com/Northern.tech/Mender/mender-stress-test-client/badges/master/pipeline.svg)](https://gitlab.com/Northern.tech/Mender/mender-stress-test-client/pipelines)

The *mender-stress-test-client* is a dummy-client intended for simple testing of
the Mender server functionality, or scale testing. It is not in any way a fully
featured client. All it does is mimic a client's API, and download the update to
/dev/null. This means that it has **no state** whatsoever, all updates are
instantly thrown away, and the client will forever function as it did once
booted up. This means that all parameters provided to the client at startup will
stay this way until the client is brought down.

## Getting Started

### Building

The `mender-stress-test-client` can be built by simply running `go build .` in
the mender-stress-test-client repository. This will put an executable binary in
the current repo. If a more universal option is wanted, the binary can be
automatically install into the `~/$GOPATH/bin` repository by running `go install
.`, and if this is already in your `$PATH` variable, then the stress-test-client
will function exactly like any other binary in your `PATH`.

### Running

If the client is already installed, run it with the default options like so:
`mender-stress-test-client -count=<device-count>`

Pass the `-h` flag for all options.

NOTE: This default setup will add one failing client by default.

## Working with the Client

NOTE: Currently there are some oddities to be had from the client, most notably:

* It has CLI options for inventory **and** for current device as well as the
current artifact, even though the latter are generally part of the inventory,
these two are separate in the stress-test-client. Meaning that the inventory set
from the CLI, will always be the one that shows up on the server, regardless of
what is set as the current device type and the current artifact name. The
command line options of current device and current artifact are used for
building the _update request to the server_, and can hence specify dummy names,
so that one can mismatch on the Artifact name, and match on the client type. In
general these flags should match up though, and hence be the same. If not
specified these will default to *test*.

* As mentioned above, in general the client has **no** state. However, this is
not wholly true, as the client will generate missing keys, and then reuse these
keys on second startup. If the client has no keys to start out with, it will
generate the number of keys needed, one for each client, and store them in the
`keys/` folder. Thus on killing the clients, and starting them back up, the keys
will still be present in the directory, and the clients will start back up with
the same keys it had on the previous run. If it needs to start more clients than
there are already keys present, it will start up one client for each key, and
then go at generating a new one for each client which is missing one.

* As mentioned, the client has no state! This implies that it can be 'updated'
with the same artifact N times, and still report the same inventory it had at
startup. This is because an update is always discarded on the client side (it is
written to /dev/null, and not parsed by mender-artifact).

* It reports the update phases, _downloading_, _installing_, _rebooting_ and
_success_ or _failure_. The time the client spends in between each of these
stages is determined by the CLI-parameter `-wait=<max-wait>`, and defaults to
`1800`. This does not mean that each device waits for `1800` seconds between
each update stage though. This is the maximum time that the client will wait in
between each update step on the update path (i.e., downloading, installing,
rebooting), where the wait is some amount of seconds on the uniform interval
`[0,max-wait)`.


## Working with the Demo Server

Following are the considerations needed when running the client with the Mender
Demo server. In order to run the Demo server with *N* clients, (this assumes the
demo server is already running), there are a few options that can be worthwhile
consideration.

* First the number of clients is specified with the `-count=<nr-devices>` flag.
  If this is a fairly large number, consider using the `-interval=<last-start>`
  flag, in order to not bombard the server with N authorization calls at once.
  This can be used to bring the clients up at an interval from
  `[time-now,time-now + last-start]`. Hence, significantly lightening the
  upstart load on the server.

* Also consider how many of the devices should fail during a simulated update.
This can be controlled with the `-failcount=<nr-failed-update-devices>` flag,
and will default to one, if not specified.

* The address of the Demo server is assumed by default to be on `localhost`, but
can be overridden by the `-backend=<server-URL>` flag.

