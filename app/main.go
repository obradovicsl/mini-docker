//go:build linux
// +build linux

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
)

// Ensures gofmt doesn't remove the imports above (feel free to remove this!)
var _ = os.Args
var _ = exec.Command

type nullReader struct{}

func (nullReader) Read(p []byte) (n int, err error) { return len(p), nil }

// Usage: your_docker.sh run <image> <command> <arg1> <arg2> ...
func main() {
	// mydocker run alpine:latest /usr/local/bin/docker-explorer echo hey

	command := os.Args[3]
	args := os.Args[4:len(os.Args)]

	// CHROOT ISOLATION

	// Create temp chroot jail directory
	chrootDir, err := os.MkdirTemp("", "mydocker-jail")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create chroot dr: %v\n", err)
		os.Exit(1)
	}

	// Compute destination path inside chroot -

	// filepath.Join - joins rootpath + command - chrootDir + command (/tmp/mydocker-jail + /usr/local/bin/docker-explorer)
	destPath := filepath.Join(chrootDir, command)
	// os.MkdirAll - creates all required directories in the destPath path - /tmp/mydocker-jail/usr and /local and /bin ....
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mkdir: %v\n", err)
		os.Exit(1)
	}

	// Copy the command binary into chroot jail
	if err := copyFile(command, destPath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to copy binary: %v\n", err)
		os.Exit(1)
	}

	// Chroot
	if err := syscall.Chroot(chrootDir); err != nil {
		fmt.Fprint(os.Stderr, "Chroot failed: %v\n", err)
		os.Exit(1)
	}

	if err := os.Chdir("/"); err != nil {
		fmt.Fprintf(os.Stderr, "Chdir failed: %v\n", err)
		os.Exit(1)
	}

	// PID Namespaces

	// NEW CODE WITH PIPED FD
	cmd := exec.Command(command, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = nullReader{}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID,
	}

	if err := cmd.Run(); err != nil {
		// fmt.Println("Fatall: ", err)
		// os.Exit(1)
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}

	// OLD CODE WITH OUTPUT
	// output, err := cmd.Output()
	// if err != nil {
	// 	fmt.Printf("Err: %v", err)
	// 	os.Exit(1)
	// }

	// fmt.Println(string(output))

}

// copyFile copies src -> dst
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Chmod(0755)
}
