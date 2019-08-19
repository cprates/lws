package box

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"time"
)

// name of the file containing the code to run in a box
const codeFileName = "code.zip"

type Manager struct {
	// directory where boxes are going to be created
	Workdir string
	boxes   map[string]*boxtype
	// errors of box's execution (not the target's error) are sent to this channel
	errors chan errBox
}

type boxtype struct {
	name       string
	image      string // path/file_name
	entryPoint string
	instances  map[string]*boxInstance
	// every box instance will be running on this host, and the manager will try to connect
	// to it every time it needs to execute an operation on an box instance
	agentHost string // TODO: make sure this is needed
}

type boxInstance struct {
	id string
}

type errBox struct {
	boxName    string
	instanceId string
	err        error
}

type BoxArg struct {
	Arg interface{}
}

// New creates an empty box Manager ready to use that will store and create new boxes under
// workdir which is advisable to be a full path.
func New(workdir string) *Manager {
	// TODO: clean up folders in 'repo' at startup. May be do the same when shutting down,
	//  listening to an OS signal
	m := &Manager{
		Workdir: workdir,
		boxes:   map[string]*boxtype{},
		errors:  make(chan errBox),
	}

	return m
}

// CreateBox creates a new box by first creating folder 'name' and stores the code in it with
// as codeFileName, then makes a copy of the base image for the given runtime and finally
// adds the file with the code to it at the root of the root.
// TODO: doc target
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

	// TODO: rollback if some of these fail. Ex: if copying the base image fails,
	//  path.Join(dstDir, fileName) must be deleted. Ex:
	// defer func() {
	//     if err != nil {
	//	       delete dir
	//      }
	// }

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
		name:       name,
		image:      imageFile,
		entryPoint: entryPoint,
		instances:  map[string]*boxInstance{},
		agentHost:  "lws", // to keep it simple at this stage, lets use a static host
	}

	return
}

// TODO: update doc
func (m *Manager) LaunchBoxInstance(boxName, instanceId string, args ...string) (err error) {

	box, exist := m.boxes[boxName]
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
		id: instanceId,
	}

	dstFolder := filepath.Join(m.Workdir, boxName, instanceId)
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

	// TODO: --same-owner is added by default when running as root but, lets make it explicit. On a container runs as root
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
		"runbox",
		append([]string{"boxing", box.agentHost, dstFolder, box.entryPoint}, args...)...,
	)
	// TODO: this should be 'redrived' to the logger I'm using
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	go func() {
		// TODO: if this throws an error, currently I have now way to know it. Try add a panic int
		//  the main of invoke cmd
		e := cmd.Run()
		fmt.Println("ERR FROM INVOKE:", e) // TODO: delete
		m.errors <- errBox{
			boxName:    box.name,
			instanceId: instance.id,
			err:        e,
		}
	}()

	// TODO: will take a few milliseconds before the service is available, and the cmd.Run only
	//  returns when the box is shutdown. Ideally the box should ping us back after
	//  starting accepting connections. Probably a good approach would be run an RPC server to
	//  receive those acks.
	//  For now, just sleep...
	time.Sleep(100 * time.Millisecond)

	return
}
