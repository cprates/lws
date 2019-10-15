package box

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// name of the file containing the code to run in a box instance
const codeFileName = "code.zip"

type manager struct {
	// directory where boxes are going to be created
	Workdir string
	boxes   sync.Map
	logger  *logrus.Logger
}

type boxtype struct {
	name          string
	image         string // path/file_name
	entryPoint    string
	instances     sync.Map
	agentHostname string // is also used as namespace name
}

type instance struct {
	id   string
	ip   string
	port string
}

// Action to be executed on the given Manager instance. The instance type is not exported
// so the user can not call these actions directly, making the pushC returned on Start
// the only way to communicate with an instance.
type Action func(m *manager)

// New creates an empty box Manager ready to use that will store and create new boxes under
// workdir which is advisable to be a full path.

// Start starts a goroutine to run a new box Manager that will store and create new boxes
// under workdir which is advisable to be a full path, and log to logOutput, returning a
// channel for external communication.
func Start(
	workdir string,
	logOutput io.Writer,
	stopC <-chan struct{},
	shutdown *sync.WaitGroup,
) chan<- Action {

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	logger.SetOutput(logOutput)

	shutdown.Add(1)
	pushC := make(chan Action)
	m := &manager{
		Workdir: workdir,
		boxes:   sync.Map{},
		logger:  logger,
	}

	go func() {
		m.Process(pushC, stopC)
		shutdown.Done()
	}()

	return pushC
}

// Process holds the main loop of the Manager which means it blocks, processes actions received
// trough reqC, stopping as soon as it reads a message from stopC.
func (m *manager) Process(reqC <-chan Action, stopC <-chan struct{}) {

	wg := sync.WaitGroup{}
	running := true
	for running {
		select {
		case action := <-reqC:
			wg.Add(1)
			go func() {
				action(m)
				wg.Done()
			}()
		case <-stopC:
			running = false
		}
	}

	m.logger.Println("Waiting for all actions to finish before proceeding to cleanup...")
	wgC := make(chan struct{}, 1)
	go func() {
		wg.Wait()
		wgC <- struct{}{}
	}()

	select {
	case <-wgC:
	case <-time.After(500 * time.Millisecond):
		m.logger.Println("Got timeout while waiting for all tasks to finish. Proceeding to cleanup")
	}
	m.cleanup()

	m.logger.Println("Shutting down Box Manager...")
}

func (m *manager) cleanup() {
	m.boxes.Range(
		func(k interface{}, v interface{}) bool {
			box := v.(*boxtype)
			box.instances.Range(
				func(ik interface{}, iv interface{}) bool {
					inst := iv.(instance)
					err := m.destroyBox(box.name, inst.id)
					if err != nil {
						m.logger.Errorf("Failed to destroy box %s:%s: %s", box.name, inst.id, err)
					}

					return true
				},
			)

			return true
		},
	)
}

// CreateBox creates a new box by first creating folder 'name' and stores the code in it with
// as codeFileName, then makes a copy of the base image for the given runtime and finally
// adds the file with the code to it at the root of the root.
// name is also used as hostname and namespace name in which the box instance will be running.
func CreateBox(
	name, imageFile, entryPoint string,
	code []byte,
	resC chan<- func() (int, error),
) Action {
	return func(m *manager) {
		var err error
		var codeSize int
		defer func() {
			resC <- func() (int, error) {
				return codeSize, err
			}
		}()

		_, exist := m.boxes.Load(name)
		if exist {
			err = errors.New("box already exist")
			return
		}

		boxDir := path.Join(m.Workdir, name)
		if _, err = os.Stat(boxDir); os.IsNotExist(err) {
			err = os.MkdirAll(boxDir, 0766)
			if err != nil {
				err = fmt.Errorf("while creating dir %q: %s", boxDir, err)
				return
			}
		}

		// makes sure we delete all the garbage if something fails
		defer func() {
			if err != nil {
				e := os.RemoveAll(boxDir)
				if e != nil {
					m.logger.Errorf("Failed to undo box creation: %s", e)
				}
			}
		}()

		// stores the code to be executed on this box under codeFileName file
		codePath := path.Join(boxDir, codeFileName)
		f, err := os.OpenFile(codePath, os.O_CREATE|os.O_WRONLY, 0766)
		if err != nil {
			err = fmt.Errorf("while opening file code at %q: %s", codePath, err)
			return
		}
		defer func() {
			e := f.Close()
			if err == nil && e != nil {
				err = fmt.Errorf("while closing file code %q: %s", codePath, e)
			}
		}()

		codeSize, err = f.Write(code)
		if err != nil {
			err = fmt.Errorf("while writing code at %q: %s", codePath, err)
			return
		}

		// create the new box with no instances
		m.boxes.Store(
			name,
			&boxtype{
				name:          name,
				image:         imageFile,
				entryPoint:    entryPoint,
				instances:     sync.Map{},
				agentHostname: name,
			},
		)
	}
}

func DestroyBox(name, instanceId string, resC chan<- func() error) Action {
	return func(m *manager) {
		err := m.destroyBox(name, instanceId)
		resC <- func() error {
			return err
		}
	}
}

func (m *manager) destroyBox(name, instanceId string) (err error) {

	b, exist := m.boxes.Load(name)
	if !exist {
		err = errors.New("box does not exist")
		return
	}
	box := b.(*boxtype)

	_, exist = box.instances.Load(instanceId)
	if !exist {
		err = errors.New("box instance does not exist")
		return
	}

	box.instances.Delete(instanceId)

	folder := filepath.Join(m.Workdir, name, instanceId)
	rmCmd := exec.Command(
		"rm",
		"-rf", folder,
	)
	err = rmCmd.Run()
	if err != nil {
		// if failed to delete the folder, it may be an instance that had a failed
		// initialisation and there is no folder to delete so, just log the error
		m.logger.Errorf("while removing instance folder %q: %s", folder, err)
		err = nil
	}

	return
}

// LaunchBoxInstance launches a new instance of a previously created boxName with the given
// intanceId, passing args to the box entry point.
//
// ip is a CIDR notation ip that will be configured on the box's network interface.
// nextHop is basically the default gateway to be used by this box instance.
//
func LaunchBoxInstance(
	name, instanceId, ip, port, nextHop string,
	resC chan<- func() error,
	args ...string,
) Action {
	return func(m *manager) {
		var err error
		defer func() {
			resC <- func() error {
				return err
			}
		}()

		b, exist := m.boxes.Load(name)
		if !exist {
			err = errors.New("box does not exist")
			return
		}
		box := b.(*boxtype)

		_, exist = box.instances.Load(instanceId)
		if exist {
			err = errors.New("box instance already exist")
			return
		}

		inst := instance{
			id:   instanceId,
			ip:   ip,
			port: port,
		}

		dstFolder := filepath.Join(m.Workdir, name, instanceId)
		if _, e := os.Stat(dstFolder); !os.IsNotExist(e) {
			// for some reason this folder already exists and it shouldn't
			err = fmt.Errorf("folder %q already exist", dstFolder)
			return
		}
		mkdirCmd := exec.Command("mkdir", "-p", dstFolder)
		err = mkdirCmd.Run()
		if err != nil {
			err = fmt.Errorf("while creating dst dir %q: %s", dstFolder, err)
			return
		}

		// TODO: --same-owner is added by default when running as root but, lets make it explicit.
		//  On a container this runs as root
		baseImageFile := filepath.Join(m.Workdir, box.image)
		extractCmd := exec.Command(
			"tar",
			"-xpf", baseImageFile,
			"-C", dstFolder,
		)
		err = extractCmd.Run()
		if err != nil {
			err = fmt.Errorf(
				"while extracting base image %q to %s: %s",
				baseImageFile, dstFolder, err,
			)
			return
		}

		codeFile := filepath.Join(m.Workdir, box.name, codeFileName)
		appFolder := filepath.Join(dstFolder, "app")
		extractCmd = exec.Command("unzip", "-q", codeFile, "-d", appFolder)
		err = extractCmd.Run()
		if err != nil {
			err = fmt.Errorf("while extracting code %q to %s: %s", codeFile, appFolder, err)
			return
		}

		// launch box instance
		cmd := exec.Command(
			"runbox.sh",
			append(
				[]string{box.entryPoint}, args...,
			)...,
		)
		cmd.Env = []string{
			"LAMBDA_NS=" + box.agentHostname,
			"HOSTNAME=" + box.agentHostname + "." + inst.id,
			"LOCAL_IP=" + inst.ip,
			"NEXT_HOP=" + nextHop,
			"FS_PATH=" + dstFolder,
			"WORK_DIR=" + os.Getenv("LWS_WORK_DIR"),
			"DEBUG=" + os.Getenv("LWS_DEBUG"),
		}

		// signal will be inspecting the output from the executed process looking for a specific
		// string that signals the rpc server is ready.
		signal := &signal{
			source: m.logger.Out,
			c:      make(chan struct{}, 1),
		}

		cmd.Stdout = signal
		cmd.Stderr = signal

		err = cmd.Start()
		if err != nil {
			err = fmt.Errorf("while starting box instance: %s", err)
			return
		}

		go func() {
			e := cmd.Wait()
			if e != nil {
				m.logger.Debugf("Box %q, instance %q exited with %q", name, instanceId, e)
			}
		}()

		select {
		case <-time.After(100 * time.Millisecond):
			err = fmt.Errorf(
				"didn't got the ready signal from Box %q, instance %q",
				name, instanceId,
			)
		case <-signal.c:
			// add instance only after we have the confirmation it was successfully added
			box.instances.Store(instanceId, inst)
		}

		//instance.waitReady()
	}
}

// waitReady checks if the box rpc server is ready to accept connections by trying to connect
// and retrying on dial errors up to 4 times.
// TODO: this is probably to delete. For some reason this does not work very well when an instance
//  reuses a previously used IP at least one minute ago (which seems to match the arp cache timeout)
//  , it keeps timing out for about 1 second (with more retries configured) before it succeeds.
//  It doesn't seem to make much sense because instead if I add a sleep of just 100ms, the call to
//  instance.Exec at llambda.go succeeds and it does the exact same net.Dial... so at this point
//  I'm not sure why it doesn't correctly.
//  Any way the approach using the stdout/stderr is working very well, the only downside is the
//  existing contract between box system and the runtime (gobox) that has to know the signal message
//  and when to send it.
func (ins *instance) waitReady() bool {

	parsedIP, _, err := net.ParseCIDR(ins.ip)
	if err != nil {
		fmt.Printf("Failed parsing the given ip %s: %s", ins.ip, err)
		return false
	}

	for i := 0; i < 40; i++ {
		conn, err := net.DialTimeout(
			"tcp", net.JoinHostPort(parsedIP.String(), ins.port), 50*time.Millisecond,
		)

		if err != nil {
			e, ok := err.(*net.OpError)

			if ok && e.Op == "dial" {
				fmt.Printf("Failed to check box status, attempt %d: %s\n", i+1, e)
				// try again after a small delay
				time.Sleep(5 * time.Millisecond)
				continue
			}
			fmt.Println("Failed checking box status:", err)
			return false
		}

		_ = conn.Close()
		return true
	}

	fmt.Println("Box seems to not be responding")
	return false
}
