#!/bin/sh

# Must be copied to the root of every base image to be executed by runbox at startup
# This is a workaround to set hosts file

echo "127.0.0.1    localhost" > /etc/hosts
echo "::1          localhost" >> /etc/hosts

# set DNS server
echo "nameserver $GATEWAY" > /etc/resolv.conf
