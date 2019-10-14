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
	"time"

	"github.com/sirupsen/logrus"
)

// name of the file containing the code to run in a box instance
const codeFileName = "code.zip"

type Manager struct {
	// directory where boxes are going to be created
	Workdir string
	boxes   map[string]*boxtype
	logger  *logrus.Logger
}

type boxtype struct {
	name          string
	image         string // path/file_name
	entryPoint    string
	instances     map[string]*boxInstance
	agentHostname string // is also used as namespace name
}

type boxInstance struct {
	id   string
	ip   string
	port string
}

// New creates an empty box Manager ready to use that will store and create new boxes under
// workdir which is advisable to be a full path.
func New(workdir string, logOutput io.Writer) *Manager {

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	logger.SetOutput(logOutput)

	m := &Manager{
		Workdir: workdir,
		boxes:   map[string]*boxtype{},
		logger:  logger,
	}

	return m
}

// Start the internal mechanisms to handle errors.
func (m *Manager) Start(stopC <-chan struct{}) {

	running := true
	for running {
		select {
		case <-stopC:
			running = false
			m.cleanup()
		}
	}

	m.logger.Println("Shutting down Box Manager...")
}

func (m *Manager) cleanup() {
	for _, box := range m.boxes {
		for _, inst := range box.instances {
			err := m.DestroyBox(box.name, inst.id)
			if err != nil {
				m.logger.Errorf("Failed to destroy box %s:%s: %s", box.name, inst.id, err)
			}
		}
	}

}

// CreateBox creates a new box by first creating folder 'name' and stores the code in it with
// as codeFileName, then makes a copy of the base image for the given runtime and finally
// adds the file with the code to it at the root of the root.
// name is also used as hostname and namespace name in which the box instance will be running.
// TODO: make it thread safe
func (m *Manager) CreateBox(name, imageFile, entryPoint string, code []byte) (n int, err error) {

	_, exist := m.boxes[name]
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

	n, err = f.Write(code)
	if err != nil {
		err = fmt.Errorf("while writing code at %q: %s", codePath, err)
		return
	}

	// create the new box with no instances
	m.boxes[name] = &boxtype{
		name:          name,
		image:         imageFile,
		entryPoint:    entryPoint,
		instances:     map[string]*boxInstance{},
		agentHostname: name,
	}

	return
}

// TODO: make it theread safe
func (m *Manager) DestroyBox(name, instanceId string) (err error) {

	box, exist := m.boxes[name]
	if !exist {
		err = errors.New("box does not exist")
		return
	}

	_, exist = box.instances[instanceId]
	if !exist {
		err = errors.New("box instance does not exist")
		return
	}

	delete(box.instances, instanceId)

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
// TODO: make it theread safe
func (m *Manager) LaunchBoxInstance(
	name, instanceId, ip, port, nextHop string,
	args ...string,
) (
	err error,
) {

	box, exist := m.boxes[name]
	if !exist {
		err = errors.New("box does not exist")
		return
	}

	instance, exist := box.instances[instanceId]
	if exist {
		err = errors.New("box instance already exist")
		return
	}

	instance = &boxInstance{
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
		err = fmt.Errorf("while extracting base image %q to %s: %s", baseImageFile, dstFolder, err)
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

	// add instance
	box.instances[instanceId] = instance

	// launch box instance
	cmd := exec.Command(
		"runbox.sh",
		append(
			[]string{box.entryPoint}, args...,
		)...,
	)
	cmd.Env = []string{
		"LAMBDA_NS=" + box.agentHostname,
		"HOSTNAME=" + box.agentHostname + "." + instance.id,
		"LOCAL_IP=" + instance.ip,
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
		m.logger.Debugf("Didn't got the ready signal from Box %q, instance %q", name, instanceId)
	case <-signal.c:
	}

	//instance.waitReady()

	return
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
func (ins *boxInstance) waitReady() bool {

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
