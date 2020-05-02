package llambda

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"time"
)

type containerManager interface {
	Create(
		name string, configF configFlags, netconfF netconfFlags, stdin, stdout, stderr *os.File,
	) (containerWrapper, error)
}

type cmanager struct {
	workdir string
}

type containerWrapper interface {
	Destroy() error
}

type cwrap struct {
	containerName string
}

type configFlags struct {
	RootFS         string
	Hostname       string
	Entrypoint     string
	LambdaName     string
	EnvRuntimePort string
	Handler        string
	UserEnvVars    []string
}

type netconfFlags struct {
	IP            string
	Bridge        string
	Gateway       string
	NameServer    string
	IFaceName     string
	PeerIFaceName string
}

var _ containerManager = (*cmanager)(nil)
var _ containerWrapper = (*cwrap)(nil)

func (c *cmanager) Create(
	name string, configF configFlags, netconfF netconfFlags, stdin, stdout, stderr *os.File,
) (containerWrapper, error) {
	config := bytes.Buffer{}
	err := configTemplate.Execute(&config, configF)
	if err != nil {
		return nil, err
	}

	netconf := bytes.Buffer{}
	err = netconfTemplate.Execute(&netconf, netconfF)
	if err != nil {
		return nil, err
	}

	configPath, err := createAndSaveFile(path.Join(c.workdir, name), "config.json", config.Bytes())
	if err != nil {
		return nil, fmt.Errorf("while storing config %q: %s", configPath, err)
	}

	netconfPath, err := createAndSaveFile(path.Join(c.workdir, name), "netconf.json", netconf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("while storing netconf %q: %s", netconfPath, err)
	}

	cmd := exec.Command(
		"./box",
		[]string{
			"--spec", configPath,
			"--netconf", netconfPath,
			"create",
			name,
		}...,
	)

	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("creating container: %s", err)
	}

	wC := make(chan error)
	go func() {
		wC <- cmd.Wait()
	}()

	select {
	case <-time.After(100 * time.Millisecond):
		return nil, errors.New("timeout creating container")
	case err = <-wC:
		if err != nil {
			return nil, fmt.Errorf("waiting for container to be created: %s", err)
		}
	}

	cmd = exec.Command(
		"./box",
		[]string{
			"start",
			name,
		}...,
	)

	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("starting container: %s", err)
	}

	go func() {
		wC <- cmd.Wait()
	}()

	select {
	case <-time.After(100 * time.Millisecond):
		return nil, errors.New("timeout starting container")
	case err = <-wC:
		if err != nil {
			return nil, fmt.Errorf("waiting for container to be starting: %s", err)
		}
	}

	return &cwrap{containerName: name}, nil
}

func createAndSaveFile(filePath, name string, data []byte) (dst string, err error) {
	err = os.MkdirAll(filePath, 0766)
	if err != nil {
		return
	}

	d := path.Join(filePath, name)
	f, err := os.Create(d)
	if err != nil {
		return
	}
	defer f.Close()

	n, err := f.Write(data)
	if err != nil {
		return
	}
	if n != len(data) {
		err = fmt.Errorf("file size: %d, wrote %d", len(data), n)
		return
	}

	dst = d
	return
}

func (c *cwrap) Destroy() error {
	cmd := exec.Command(
		"./box",
		[]string{
			"destroy",
			c.containerName,
		}...,
	)

	return cmd.Start()
}
