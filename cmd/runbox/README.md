#runbox

Is a very simple set of bash scripts to help setting up a linux container.
It is used by *llambda* as an utility to run lambdas on an isolated environment from the host.  
*runbox* is currently composed by two scripts, the main one, runbox.sh, which works like a
bootstrap setting up some base configs like hostname, DNS and network interfaces, etc, and
launching the second stage of the process on a separate namespace.  
The second stage, stage2.sh, is responsible for all the configs on the 'container' side like
mounting points, chroot to the given *FS_PATH* and execute the entry point.


## Configuration

Mandatory environment variables:

- *LAMBDA_NS*: namespace's name to be used in this box
- *FS_PATH*: full path of the file system to be used by this box. Ex.: ```/home/user/somefs```
- *LOCAL_IP*: ip in *CIDR notation* to be configured on the box. Ex.: ```172.17.0.2/30```
- *HOSTNAME*: hostname to be configured on the box
- *NEXT_HOP*: ip in *CIDR notation* of the veth on the host side. Ex.: ```172.17.0.1/30```

Optional environment variables:
- *IF_BRIDGE*: specifies the bridge interface on the host to link the veth to. Defaults to ```br0```
- *DEBUG*: when set to *1*, prints to stdout all executed commands and their outputs when any


##Examples

```
sudo LAMBDA_NS=runbox \
    FS_PATH=/path/to/some/fs \
    LOCAL_IP=172.17.0.2/30 \
    HOSTNAME=runboxhost \
    NEXT_HOP=172.17.0.1/30 \
    IF_BRIDGE=docker0 \
    WORK_DIR=. \
    DEBUG=0 \
    ./runbox.sh ps aux
```
