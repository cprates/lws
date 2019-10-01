#!/bin/sh

set -e
test "$DEBUG" -eq 1 && set -x

IF_BRIDGE=br0
IF_HOST=eth0

ip link add name ${IF_BRIDGE} type bridge
ip link set ${IF_BRIDGE} up

# get rid of the address set by docker before adding the device to the bridge
ip addr flush dev ${IF_HOST}
ip link set ${IF_HOST} master ${IF_BRIDGE}

ip addr add ${LWS_IP}/${LWS_NETWORK_BITS} dev ${IF_BRIDGE}

ip route add default via ${LWS_DOCKER_GW}

# execute the target
exec /lws/lws
