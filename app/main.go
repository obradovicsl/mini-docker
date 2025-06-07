//go:build linux
// +build linux

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	if len(os.Args) < 4 || os.Args[1] != "run" {
		fmt.Fprintf(os.Stderr, "\nuse: run <image> <command> <arg1> <arg2> .... <argN>")
		os.Exit(1)
	}

	command := os.Args[3]
	args := os.Args[4:len(os.Args)]

	// get image name and version
	imageName, imageVersion, err := getImageNameAndVersion(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to extract image: %v\n", err)
		os.Exit(1)
	}

	println(imageName, imageVersion)
	token, err := getAuthToken(imageName)
	if err != nil {
		fmt.Fprint(os.Stderr, "Auth failed: ", err)
		os.Exit(1)
	}
	fmt.Println("Token: ", token)

	// CHROOT ISOLATION

	// Create temp chroot jail directory
	chrootDir, err := os.MkdirTemp("", "mydocker-jail")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create chroot dr: %v\n", err)
		os.Exit(1)
	}

	// ONLY if it doesn't exist already in the image

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

func getAuthToken(imageName string) (string, error) {
	// Ako je image bez namespace-a, pretpostavljamo "library/"
	if !strings.Contains(imageName, "/") {
		imageName = "library/" + imageName
	}

	authURL := fmt.Sprintf(
		"https://auth.docker.io/token?service=registry.docker.io&scope=repository:%s:pull",
		imageName,
	)

	resp, err := http.Get(authURL)
	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status from auth: %s", resp.Status)
	}

	var data struct {
		Token string `json:"token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	return data.Token, nil
}

func getImageNameAndVersion(imageString string) (string, string, error) {
	if !strings.Contains(imageString, ":") {
		return imageString, "latest", nil
	}
	parts := strings.Split(imageString, ":")
	imageName, imageVersion := parts[0], parts[1]

	return imageName, imageVersion, nil
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
