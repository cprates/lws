package llambda

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/rpc"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"text/template"
	"time"

	"github.com/aws/aws-lambda-go/lambda/messages" // TODO: instead of adding the dependency, replicate the struct?
	log "github.com/sirupsen/logrus"

	"github.com/cprates/lws/pkg/list"
)

type funcInstancer interface {
	Exec(arg interface{}) (reply *messages.InvokeResponse, err error)
	Shutdown() (err error)
	ID() string
	LifeDeadline() time.Time
	IP() net.IP
}

type funcInstance struct {
	parent       *function
	id           string
	ip           net.IP
	lifeDeadline time.Time
	container    containerWrapper
}

// TODO: vale a pena criar uma interface para isto?
type function struct {
	folder            string
	runtimeImage      string // folder/to/image/runtime.ext
	runtimeEntryPoint string // ex.: /bin/runbox
	usercodeFile      string // folder/to/usercode.zip
	description       string
	envVars           map[string]string
	handler           string
	lrn               string
	memorySize        int
	name              string
	publish           bool
	revID             string
	role              string
	timeout           int // seconds
	version           string
	idleInstances     *list.List
	containerManager  containerManager
	logger            *log.Entry
}

var (
	netconfTemplate *template.Template
	configTemplate  = template.New("config")
)

var _ funcInstancer = (*funcInstance)(nil)

var (
	runtimeListenPort    = "50127"
	containerDialTimeout = 50 * time.Millisecond
)

var (
	configStr = `
{
  "ociVersion": "1.0.1",

  "root": {
    "path": "{{.RootFS}}"
  },

  "hostname": "{{.Hostname}}",

  "process": {
    "args": [
      "{{.Entrypoint}}"
    ],
    "env": [
      "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
      "TERM=xterm",
      "HOME=/root",
      "AWS_LAMBDA_FUNCTION_NAME={{.LambdaName}}",
      "GOBOX_PORT={{.EnvRuntimePort}}",
      {{range .UserEnvVars}}"{{.}}",{{end}}
      "HANDLER={{.Handler}}"
    ],
    "cwd": "/app"
  }
}`

	netconfStr = `
{
  "loopback_name": "lo",
  "model": {
    "type": "bridge",
    "bridge_name": "{{.Bridge}}"
  },
  "interfaces": [
    {
      "type": "veth",
      "name": "{{.IFaceName}}",
      "peer_name": "{{.PeerIFaceName}}",
      "ip": "0.0.0.0/0",
      "peer_ip":  "{{.IP}}",
      "routes": [
        {
          "subnet": "0.0.0.0/0",
          "gateway": "{{.Gateway}}"
        }
      ]
    }
  ],
  "dns": {
    "nameservers": [
      "{{.NameServer}}"
    ],
    "domain": "lws",
    "search": [
      "lws",
      "lws.local"
    ]
  }
}`
)

func initTemplates() (err error) {
	configTemplate, err = template.New("config").Parse(configStr)
	if err != nil {
		return err
	}

	netconfTemplate, err = template.New("netconf").Parse(netconfStr)
	if err != nil {
		return err
	}

	return
}

// Init initialises the function by creating its folder and storing its code.
func (f *function) Init(code io.Reader) (int64, error) {
	_, err := os.Stat(f.folder)
	if err != nil {
		if !os.IsNotExist(err) {
			return 0, fmt.Errorf("unable to get folder %q stats: %s", f.folder, err)
		}

		err = os.MkdirAll(f.folder, 0766)
		if err != nil {
			return 0, fmt.Errorf("unable to create folder %q: %s", f.folder, err)
		}
	}

	// makes sure we delete all the garbage if something fails
	defer func() {
		if err != nil {
			e := os.RemoveAll(f.folder)
			if e != nil {
				f.logger.Errorf("failed to undo function creation: %s", e)
			}
		}
	}()

	// stores the code to be executed on this container under usercodeFile
	fCode, err := os.OpenFile(f.usercodeFile, os.O_CREATE|os.O_WRONLY, 0766)
	if err != nil {
		return 0, fmt.Errorf("opening file code at %q: %s", f.usercodeFile, err)
	}
	defer func() {
		e := fCode.Close()
		if err == nil && e != nil {
			err = fmt.Errorf("closing file code %q: %s", f.usercodeFile, e)
		}
	}()

	codeSize, err := io.Copy(fCode, code)
	if err != nil {
		return codeSize, fmt.Errorf("writing code to %q: %s", f.usercodeFile, err)
	}

	return codeSize, nil
}

func (f *function) Instance(
	id string, ip net.IP, ipNet *net.IPNet, stdin, stdout, stderr *os.File, runtimePort string,
) (funcInstancer, error) {

	rootFS, err := f.prepareInstanceFS(id)
	if err != nil {
		return nil, fmt.Errorf("preparing container fs: %s", err)
	}

	mask, _ := ipNet.Mask.Size()
	var userEnvVars []string
	for k, v := range f.envVars {
		userEnvVars = append(userEnvVars, k+"="+v)
	}
	// hostname of a function's instance should contain the function name to help debugging
	container, err := f.containerManager.Create(
		id,
		configFlags{
			RootFS:         rootFS,
			Hostname:       id,
			Entrypoint:     f.runtimeEntryPoint,
			LambdaName:     f.name,
			EnvRuntimePort: runtimePort,
			Handler:        f.handler,
			UserEnvVars:    userEnvVars,
		},
		netconfFlags{
			IP:            ip.String() + "/" + strconv.Itoa(mask),
			Bridge:        os.Getenv("LWS_IF_BRIDGE"), // TODO: move this to a general config struct
			Gateway:       os.Getenv("LWS_IP"),
			NameServer:    os.Getenv("LWS_NAMESERVER"),
			IFaceName:     id + "_0",
			PeerIFaceName: id + "_1",
		},
		stdin,
		stdout,
		stderr,
	)
	if err != nil {
		return nil, fmt.Errorf("creating container: %s", err)
	}

	return &funcInstance{
		parent:       f,
		id:           id,
		ip:           ip,
		lifeDeadline: time.Now().Add(time.Duration(f.timeout) * time.Second),
		container:    container,
	}, nil
}

func (f *function) prepareInstanceFS(id string) (string, error) {
	dstFolder := filepath.Join(f.folder, id, "fs")
	if _, e := os.Stat(dstFolder); !os.IsNotExist(e) {
		// for some reason this folder already exists and it shouldn't
		return "", fmt.Errorf("folder %q already exist", dstFolder)
	}

	if err := os.MkdirAll(dstFolder, 0766); err != nil {
		return "", fmt.Errorf("while creating dst dir %q: %s", dstFolder, err)
	}

	// --same-owner is added by default when running as root but, lets make it explicit.
	// On a container this runs as root
	// TODO: use a lib instead of tar cmd
	extractCmd := exec.Command(
		"tar",
		"-xpf", f.runtimeImage,
		"-C", dstFolder,
	)
	if err := extractCmd.Run(); err != nil {
		return "", fmt.Errorf(
			"while extracting base image %q to %s: %s",
			f.runtimeImage, dstFolder, err,
		)
	}

	appFolder := filepath.Join(dstFolder, "app")
	// TODO: use a lib instead of unzip cmd
	extractCmd = exec.Command("unzip", "-q", f.usercodeFile, "-d", appFolder)
	if err := extractCmd.Run(); err != nil {
		return "", fmt.Errorf("while extracting code %q to %s: %s", f.usercodeFile, appFolder, err)
	}

	return dstFolder, nil
}

func (f *funcInstance) Exec(arg interface{}) (reply *messages.InvokeResponse, err error) {
	logger := f.parent.logger

	instanceAddr := net.JoinHostPort(f.ip.String(), runtimeListenPort)

	logger.Debugf("Connecting to %s[%s] at %s...\n", f.parent.name, f.id, instanceAddr)

	// instead of keeping a connection alive, connect to the container every time we need
	conn, err := f.waitReady()
	if err != nil {
		err = fmt.Errorf("dial to container at %q failed: %s", instanceAddr, err)
		return
	}
	c := rpc.NewClient(conn)

	defer func() {
		e := c.Close()
		if err == nil {
			err = e
		}
	}()

	// update deadline
	f.lifeDeadline = time.Now().Add(time.Duration(f.parent.timeout) * time.Second)

	logger.Debugf("Executing agent on container %s[%s]\n", f.parent.name, f.id)
	reply = &messages.InvokeResponse{}
	err = c.Call("Server.Exec", arg, reply)
	if err != nil {
		err = fmt.Errorf("call to container failed: %s", err)
		return
	}

	return
}

// Shutdown is for shutdown a lambda's instance by calling Shutdown method on the remote
// instance using the RPC protocol.
func (f *funcInstance) Shutdown() (err error) {
	defer func() {
		e := f.container.Destroy()
		if e != nil {
			e = fmt.Errorf("destroying container: %s", e)
			if err != nil {
				e = fmt.Errorf("%s, AND %s", err, e)
			}
		}
		err = e

		e = os.RemoveAll(path.Join(f.parent.folder, f.id))
		if e == nil {
			return
		}
		e = fmt.Errorf("deleting container folder: %s", e)
		if err != nil {
			e = fmt.Errorf("%s, AND %s", err, e)
		}
		err = e
	}()

	instanceAddr := net.JoinHostPort(f.ip.String(), runtimeListenPort)
	conn, err := net.DialTimeout("tcp", instanceAddr, containerDialTimeout)
	if err != nil {
		err = fmt.Errorf("dialing to %q: %s", instanceAddr, err)
		return
	}
	c := rpc.NewClient(conn)

	defer func() {
		e := c.Close()
		if err == nil {
			err = e
		}
	}()

	err = c.Call("Server.Shutdown", "", nil)
	if err != nil {
		err = fmt.Errorf("rpc call: %s", err)
		return
	}

	return
}

func (f *funcInstance) ID() string {
	return f.id
}

func (f *funcInstance) LifeDeadline() time.Time {
	return f.lifeDeadline
}

func (f *funcInstance) IP() net.IP {
	return f.ip
}

func (f *funcInstance) waitReady() (net.Conn, error) {
	logger := f.parent.logger

	addr := net.JoinHostPort(f.ip.String(), runtimeListenPort)
	for i := 0; i < 5; i++ {
		conn, err := net.DialTimeout("tcp", addr, containerDialTimeout)
		if err != nil {
			e, ok := err.(*net.OpError)

			if ok && e.Op == "dial" {
				logger.Debugf("Failed to check runtime status, attempt %d: %s. Retrying...\n", i+1, err)
				// try again after a small delay
				time.Sleep(5 * time.Millisecond)
				continue
			}
			return nil, fmt.Errorf("checking runtime status after %d attempts: %s", i, err)
		}

		return conn, nil
	}

	return nil, errors.New("runtime seems to not be responding")
}
