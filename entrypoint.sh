#!/usr/bin/env sh

args="$@"

[ ! -z "$COUNT" ] && args="$args -count $COUNT"
[ ! -z "$TENANT_TOKEN" ] && args="$args -tenant $TENANT_TOKEN"
[ ! -z "$BACKEND_URL" ] && args="$args -backend $BACKEND_URL"
[ ! -z "$POLL_FREQ" ] && args="$args -pollfreq $POLL_FREQ"
[ ! -z "$INVENTORY_FREQ" ] && args="$args -invfreq $INVENTORY_FREQ"

/mender-stress-test-client $args

