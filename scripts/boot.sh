#!/bin/sh

set -e
test "$LWS_DEBUG" -eq 1 && set -x

# update path with the given working dir where runbox is stored
PATH=$PATH:$LWS_WORK_DIR

ip link add name ${LWS_IF_BRIDGE} type bridge
ip link set ${LWS_IF_BRIDGE} up

# get rid of the address set by docker before adding the device to the bridge
ip addr flush dev ${LWS_IF_HOST}
ip link set ${LWS_IF_HOST} master ${LWS_IF_BRIDGE}

ip addr add ${LWS_IP}/${LWS_NETWORK_BITS} dev ${LWS_IF_BRIDGE}

ip route add default via ${LWS_DOCKER_GW}

# execute the target
exec /lws/lws
