package llambda

import (
	"fmt"
	"os"
	"path"
	"strconv"
)

type stdIOFiles struct {
	Stdin  *os.File
	Stdout *os.File
	Stderr *os.File
}

// this is a hack to store logs with the right uid/gid on the host when the log folder is bind
// to a host's folder. If no USER_UID or HOST_GID is provided, defaults to 0.
func prepareIOFiles(fName, instanceID string) (*stdIOFiles, error) {
	uid := 0
	if id := os.Getenv("USER_UID"); id != "" {
		i, err := strconv.Atoi(id)
		if err == nil {
			uid = i
		}
	}

	gid := 0
	if id := os.Getenv("GROUP_UID"); id != "" {
		i, err := strconv.Atoi(id)
		if err == nil {
			gid = i
		}
	}

	// base folder
	basePath := "/var/log/lambda"
	err := os.MkdirAll(basePath, 0766)
	if err != nil {
		return nil, fmt.Errorf("creating logs base folder: %s", err)
	}
	err = os.Chown(basePath, uid, gid)
	if err != nil {
		return nil, fmt.Errorf("chown logs base folder: %s", err)
	}

	// instance folder
	functionLogPath := path.Join(basePath, fName)
	err = os.MkdirAll(functionLogPath, 0766)
	if err != nil {
		return nil, fmt.Errorf("creating function log folder: %s", err)
	}
	err = os.Chown(functionLogPath, uid, gid)
	if err != nil {
		return nil, fmt.Errorf("chown function log folder: %s", err)
	}

	instanceLogPath := path.Join(functionLogPath, instanceID)
	err = os.MkdirAll(instanceLogPath, 0766)
	if err != nil {
		return nil, fmt.Errorf("creating instance log folder: %s", err)
	}
	err = os.Chown(instanceLogPath, uid, gid)
	if err != nil {
		return nil, fmt.Errorf("chown instance log folder: %s", err)
	}

	// files
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	filePath := path.Join(instanceLogPath, "stdout")
	stdout, err := os.OpenFile(filePath, flags, 0664)
	if err != nil {
		return nil, fmt.Errorf("creating stdout file: %s", err)
	}
	err = os.Chown(filePath, uid, gid)
	if err != nil {
		return nil, fmt.Errorf("chown stdout file: %s", err)
	}

	filePath = path.Join(instanceLogPath, "stderr")
	stderr, err := os.OpenFile(filePath, flags, 0664)
	if err != nil {
		return nil, fmt.Errorf("creating stderr file: %s", err)
	}
	err = os.Chown(filePath, uid, gid)
	if err != nil {
		return nil, fmt.Errorf("chown stderr file: %s", err)
	}

	// TODO: until a 'console' dev is not supported, pass stdin file to make it
	//  possible to run a console
	stdin := os.Stdin

	return &stdIOFiles{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	}, nil
}
