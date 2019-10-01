#!/bin/sh

set -e
test "$DEBUG" -eq 1 && set -x

PID=$$
IF_HOST=h_$PID
export IF_BOX=b_$PID
: ${IF_BRIDGE:=br0}

ip netns add $LAMBDA_NS
trap "ip netns del $LAMBDA_NS" EXIT

ip link add name $IF_HOST type veth peer name $IF_BOX
ip link set $IF_BOX netns $LAMBDA_NS

# hosts file
echo "127.0.0.1    localhost" > $FS_PATH/etc/hosts
echo "::1          localhost" >> $FS_PATH/etc/hosts
echo "$LOCAL_IP    $HOSTNAME" >> $FS_PATH/etc/hosts

# DNS server
echo "nameserver 8.8.8.8" > $FS_PATH/etc/resolv.conf

ip link set $IF_HOST up
ip link set $IF_HOST master $IF_BRIDGE
ip addr add $NEXT_HOP dev $IF_BRIDGE


ip netns exec $LAMBDA_NS unshare -iumpf /bin/stage2.sh $@