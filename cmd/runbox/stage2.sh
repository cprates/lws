#!/bin/sh

set -e
test "$DEBUG" -eq 1 && set -x

hostname $HOSTNAME

mount -t proc proc $FS_PATH/proc
trap "umount $FS_PATH/proc" EXIT
mount -t tmpfs tmpfs $FS_PATH/tmp
trap "umount $FS_PATH/tmp" EXIT

# create device nodes
mount -n -t tmpfs none $FS_PATH/dev

mknod -m 622 $FS_PATH/dev/console c 5 1
mknod -m 666 $FS_PATH/dev/null c 1 3
mknod -m 666 $FS_PATH/dev/zero c 1 5
mknod -m 666 $FS_PATH/dev/ptmx c 5 2
mknod -m 666 $FS_PATH/dev/tty c 5 0
mknod -m 444 $FS_PATH/dev/random c 1 8
mknod -m 444 $FS_PATH/dev/urandom c 1 9

# TODO: boxes are sharing stdin, stdout, etc with host and it shouldn't. Fix this later.
#ln -s /proc/self/fd $FS_PATH/dev/fd
#ln -s /proc/self/fd/0 $FS_PATH/dev/stdin
#ln -s /proc/self/fd/1 $FS_PATH/dev/stdout
#ln -s /proc/self/fd/2 $FS_PATH/dev/stderr
#ln -s /proc/kcore $FS_PATH/dev/core

mkdir $FS_PATH/dev/pts
mkdir $FS_PATH/dev/shm
mount -t devpts -o gid=4,mode=620 none $FS_PATH/dev/pts
mount -t tmpfs none $FS_PATH/dev/shm


ip link set lo up
ip link set $IF_BOX name eth0 up
ip addr add $LOCAL_IP dev eth0

ip route add default via $(echo $NEXT_HOP | cut -d/ -f1)


chroot $FS_PATH $@
