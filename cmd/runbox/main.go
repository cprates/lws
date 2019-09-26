package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func main() {
	switch os.Args[1] {
	case "boxing":
		boxing()
	case "run":
		run()
	default:
		panic(`Usage:
  ./runbox boxing ip_gateway hostname /path/to/fs /path/to/entrypoint <...args>
`,
		)
	}
}

func boxing() {
	cmd := exec.Command("/proc/self/exe", append([]string{"run"}, os.Args[3:]...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:   syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
		Unshareflags: syscall.CLONE_NEWNS,
	}
	cmd.Env = []string{"GATEWAY=" + os.Args[2]}

	must(cmd.Run())
}

func run() {
	setup()

	cmd := exec.Command(os.Args[4], os.Args[5:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	must(cmd.Run())

	must(syscall.Unmount("proc", 0))
}

func setup() {

	hostname := os.Args[2]
	fmt.Printf("setting hostname %q\n", hostname)
	must(syscall.Sethostname([]byte(hostname)))

	fs := os.Args[3]
	fmt.Printf("chrouting to %q\n", fs)
	must(syscall.Chroot(fs))

	must(os.Chdir("/"))
	must(syscall.Mount("proc", "proc", "proc", 0, ""))

	// init script
	// TODO: probably should be configurable
	cmd := exec.Command("/bin/sh", "-c", "/init.sh")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	must(cmd.Run())
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
