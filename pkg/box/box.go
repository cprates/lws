package box

import (
	"errors"
	"fmt"
	"net/rpc"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// name of the file containing the code to run in a box
const codeFileName = "code.zip"

type Manager struct {
	hostname string
	// directory where boxes are going to be created
	workdir string
	boxes   map[string]*boxInstance
	// errors of box's execution (not the target's error) are sent to this channel
	errors chan errBox
}

type boxtype_ struct {
	name      string
	image     string // file name
	target    string // exec name
	instances []*boxinstance
}

type boxInstance struct {
	addr string
	id   string
}

type errBox struct {
	boxId string
	err   error
}

type Args struct{}

func New(hostname, workdir string) *Manager {
	// TODO: clean up folders in 'repo' at startup. May be do the same when shutting down,
	//  listening to an OS signal
	m := &Manager{
		hostname: hostname,
		workdir:  workdir,
		boxes:    map[string]*boxInstance{},
		errors:   make(chan errBox),
	}

	return m
}

// Box creates a new box by first creates folder 'name' and stores the code in it with
// as codeFileName, then makes a copy of the base image for the given runtime and finally
// adds the file with the code to it at the root of the root.
// TODO: doc target
func (m *Manager) Box(name, target, imageFile string, code []byte) (n int, err error) {

	_, exist := m.boxes[name]
	if exist {
		err = errors.New("box already exist")
		return
	}

	boxDir := path.Join(m.workdir, name)
	if _, err = os.Stat(boxDir); os.IsNotExist(err) {
		err = os.MkdirAll(boxDir, 0766)
		if err != nil {
			err = fmt.Errorf("while creating dir: %s", err)
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

	f, err := os.OpenFile(path.Join(boxDir, codeFileName), os.O_CREATE|os.O_WRONLY, 0766)
	if err != nil {
		err = fmt.Errorf("while opening file code: %s", err)
		return
	}
	defer func() {
		e := f.Close()
		if err == nil && e != nil {
			err = fmt.Errorf("while closing file code: %s", e)
		}
	}()

	n, err = f.Write(code)
	if err != nil {
		err = fmt.Errorf("while writing code: %s", err)
		return
	}

	// create the new box with no instances
	m.boxes[name] = &boxtype{
		name:   name,
		image:  imageFile,
		target: target,
	}

	// TODO: delete copy.go
	//err = copyFile(
	//	filepath.Join(dstDir, containerFilename),
	//	filepath.Join(i.codePath, Runtime.imageFile[runtime]),
	//	0644,
	//)
	//if err != nil {
	//	err = fmt.Errorf("while copying base image: %s", err)
	//	return
	//}

	//cwd, _ := os.Getwd()
	//err = os.Chdir(dstDir)
	//if err != nil {
	//	err = fmt.Errorf("while changing dir: %s", err)
	//	return
	//}
	//// restore CWD, ignoring errors, its not critical
	//defer func() {
	//	_ = os.Chdir(cwd)
	//}()

	//cmd := exec.Command("tar", "-rf", containerFilename, fileName)
	//err = cmd.Run()
	//if err != nil {
	//	err = fmt.Errorf("while adding file code to container: %s", err)
	//	return
	//}

	return
}

func (m *Manager) freeInstance(boxName string) *boxinstance {

	box, _ := m.boxes[boxName]
	if len(box.instances) == 0 {
		return nil
	}

	// TODO: really look at a free instance
	return box.instances[0]
}

// TODO: update doc
// search for a free container or create a new one if all are busy or not created yet.
// name should be the lambda's name.
// Extract code to app/
// this launch the box
func (m *Manager) pullInstance(boxName string) (instance *boxinstance, err error) {

	box, exist := m.boxes[boxName]
	if !exist {
		err = errors.New("box does not exist")
		return
	}

	instance = m.freeInstance(boxName)
	if instance != nil {
		return
	}

	// TODO: handle containers allocation
	ringBufferIndex := 0
	instance = &boxinstance{
		addr: m.hostname + ":" + strconv.Itoa(50123), // TODO: sequential ports on a ring buffer
		id:   strconv.Itoa(ringBufferIndex),
	}

	// TODO: should check if the folder exists and error if it does?
	dstFolder := filepath.Join(m.workdir, boxName, strconv.Itoa(ringBufferIndex))
	mkdirCmd := exec.Command("mkdir", "-p", dstFolder)
	err = mkdirCmd.Run()
	if err != nil {
		err = fmt.Errorf("while creating dst dir %q: %s", dstFolder, err)
		return
	}

	// TODO: --same-owner is added by default when running as root but, lets make it explicit. On the container will run as root
	baseImageFile := filepath.Join(m.workdir, box.image)
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

	codeFile := filepath.Join(m.workdir, box.name, codeFileName)
	appFolder := filepath.Join(dstFolder, "app")
	extractCmd = exec.Command("unzip", "-q", codeFile, "-d", appFolder)
	err = extractCmd.Run()
	if err != nil {
		err = fmt.Errorf("while extracting code %q to %s: %s", codeFile, appFolder, err)
		return
	}

	// add container
	box.instances = append(box.instances, instance)

	cwd, _ := os.Getwd()

	// TODO: test with a wrong handler
	fmt.Println("CHROUTING TO:", filepath.Join(cwd, m.workdir, boxName, strconv.Itoa(ringBufferIndex))) // TODO: delete

	fs := filepath.Join(cwd, m.workdir, boxName, strconv.Itoa(ringBufferIndex)) + "/"
	entrypoint := filepath.Join(fs, "app", box.target)

	// launch container
	cmd := exec.Command("invoke", instance.addr, fs, entrypoint)
	// TODO: this should be 'redrived' to the logger I'm using
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:   syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
		Unshareflags: syscall.CLONE_NEWNS,
	}

	go func() {
		// TODO: if this throws an error, currently I have now way to know it. Try add a panic int
		//  the main of invoke cmd
		e := cmd.Run()
		fmt.Println("ERR FROM INVOKE:", e)
		m.errors <- errContainer{
			boxId: instance.id,
			err:   e,
		}
	}()

	// TODO: will take a few milliseconds before the service is available, and the cmd.Run only
	//  returns when the container is shutdown. Ideally the container should ping us back after
	//  starting accepting connections. Probably a good approach would be run an RPC server to
	//  receive those acks.
	//  For now, just sleep...
	time.Sleep(500 * time.Millisecond)

	return
}

//func (m *Manager) pushContainer(cont *container, ttl time.Duration) {
//	// update the shut at time for this container
//	cont.shutAt = time.Now().UTC().Add(ttl)
//	// add it to the free containers list
//	m.boxes[cont.name].PushBack(cont)
//}

func (m *Manager) Exec(boxName string) (err error) {

	cont, err := m.pullInstance(boxName)
	if err != nil {
		return
	}

	fmt.Println("Connecting...") // TODO: delete
	// instead of keeping a connection alive, connect to the container every time we need
	c, err := rpc.Dial("tcp", cont.addr)
	if err != nil {
		err = fmt.Errorf("dial to container failed: %s", err)
		return
	}
	defer func() {
		e := c.Close()
		if err == nil {
			err = e
		}
	}()

	fmt.Println("Calling...") // TODO: delete
	res := ""
	err = c.Call("Server.Exec", &Args{}, &res)
	if err != nil {
		err = fmt.Errorf("call to container failed: %s", err)
		return
	}

	fmt.Println("Response:", res) // TODO: delete

	// recycle container
	//m.pushContainer(cont, ttl)

	return
}
