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
	basePath := path.Join("/var/log/lambda", fName)
	err := os.MkdirAll(basePath, 0766)
	if err != nil {
		return nil, fmt.Errorf("creating logs base folder for %s[%s]: %s", fName, instanceID, err)
	}
	err = os.Chown(basePath, uid, gid)
	if err != nil {
		return nil, fmt.Errorf("chown logs base folder for %s[%s]: %s", fName, instanceID, err)
	}

	// instance folder
	logPath := path.Join(basePath, instanceID)
	err = os.MkdirAll(logPath, 0766)
	if err != nil {
		return nil, fmt.Errorf("creating logs folder for %s[%s]: %s", fName, instanceID, err)
	}
	err = os.Chown(logPath, uid, gid)
	if err != nil {
		return nil, fmt.Errorf("chown logs folder for %s[%s]: %s", fName, instanceID, err)
	}

	// files
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	filePath := path.Join(logPath, "stdout")
	stdout, err := os.OpenFile(filePath, flags, 0664)
	if err != nil {
		return nil, fmt.Errorf("creating stdout file for %s[%s]: %s", fName, instanceID, err)
	}
	err = os.Chown(filePath, uid, gid)
	if err != nil {
		return nil, fmt.Errorf("chown stdout file for %s[%s]: %s", fName, instanceID, err)
	}

	filePath = path.Join(logPath, "stderr")
	stderr, err := os.OpenFile(filePath, flags, 0664)
	if err != nil {
		return nil, fmt.Errorf("creating stderr file for %s[%s]: %s", fName, instanceID, err)
	}
	err = os.Chown(filePath, uid, gid)
	if err != nil {
		return nil, fmt.Errorf("chown stderr file for %s[%s]: %s", fName, instanceID, err)
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
