#!/bin/sh

set -e
test "$DEBUG" -eq 1 && set -x

cleanup() {
    umount $FS_PATH/proc || true
    umount $FS_PATH/tmp || true
}

hostname $HOSTNAME

mount -t proc proc $FS_PATH/proc
trap cleanup EXIT
mount -t tmpfs tmpfs $FS_PATH/tmp

# create device nodes
mount -n -t tmpfs none $FS_PATH/dev

mknod -m 622 $FS_PATH/dev/console c 5 1
mknod -m 666 $FS_PATH/dev/null c 1 3
mknod -m 666 $FS_PATH/dev/zero c 1 5
mknod -m 666 $FS_PATH/dev/ptmx c 5 2
mknod -m 666 $FS_PATH/dev/tty c 5 0
mknod -m 444 $FS_PATH/dev/random c 1 8
mknod -m 444 $FS_PATH/dev/urandom c 1 9

ln -s /proc/self/fd/0 $FS_PATH/dev/stdin
ln -s /proc/self/fd/1 $FS_PATH/dev/stdout
ln -s /proc/self/fd/2 $FS_PATH/dev/stderr

mkdir $FS_PATH/dev/pts
mkdir $FS_PATH/dev/shm
mount -t devpts -o gid=4,mode=620 none $FS_PATH/dev/pts
mount -t tmpfs none $FS_PATH/dev/shm


ip link set lo up
ip link set $IF_BOX name eth0 up
ip addr add $LOCAL_IP dev eth0

ip route add default via $(echo $NEXT_HOP | cut -d/ -f1)
# Because reuse of IPs may happen very frequently we need to announce to remote machines
# that they need to update their APR tables, by sending Gratuitous ARP responses.
# It may take a while so, leave it running in background to avoid delaying the lambda bootup.
# Plus, this is something related to llambda, if we want to use this on a more generic way,
# may be it should has an extra flag to execute this.
arping -c 2 -w 1 -I eth0 -s $(echo $LOCAL_IP | cut -d/ -f1) 255.255.255.255 > /dev/null &


chroot $FS_PATH \
  env - \
  PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
  HOSTNAME=$HOSTNAME \
  HOME=/root \
  $@
