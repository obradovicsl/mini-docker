//go:build linux
// +build linux

package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

type ManifestList struct {
	SchemaVersion int    `json:"schemaVersion"`
	MediaType     string `json:"mediaType"`
	Manifests     []struct {
		MediaType string `json:"mediaType"`
		Digest    string `json:"digest"`
		Platform  struct {
			Architecture string `json:"architecture"`
			OS           string `json:"os"`
		} `json:"platform"`
	} `json:"manifests"`
}

type Manifest struct {
	Layers []struct {
		Digest    string `json:"digest"`
		Size      int    `json:"size"`
		MediaType string `json:"mediaType"`
	} `json:"layers"`
}

func main() {

	// Check input validity
	if len(os.Args) < 4 || os.Args[1] != "run" {
		fmt.Fprintf(os.Stderr, "\nuse: run <image> <command> <arg1> <arg2> .... <argN>")
		os.Exit(1)
	}

	// Get command and arguments
	command := os.Args[3]
	args := os.Args[4:len(os.Args)]

	// Get image name and version
	imageName, imageVersion, err := getImageNameAndVersion(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to extract image: %v\n", err)
		os.Exit(1)
	}

	// Get Auth token
	token, err := getAuthToken(imageName)
	if err != nil {
		fmt.Fprint(os.Stderr, "Auth failed: ", err)
		os.Exit(1)
	}

	// Get Manifest for Image
	manifest, err := getImageManifest(imageName, imageVersion, token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Get manifest failed: ", err)
		os.Exit(1)
	}

	// CHROOT ISOLATION

	// Create temp chroot jail directory
	chrootDir, err := os.MkdirTemp("", "mydocker-jail")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create chroot dr: %v\n", err)
		os.Exit(1)
	}

	defer os.RemoveAll(chrootDir)

	// Extract all image layers inside chroot directory
	err = getAllLayers(manifest, imageName, token, chrootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Get all layers failed: ", err)
		os.Exit(1)
	}


	// Check if command (its path) exist inside chroot directory - if it comes with the image
	destPath := filepath.Join(chrootDir, command)
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		// If the path of command doesn't exist inside our chroot - we have to copy it from host 

		// Compute destination path inside chroot:
		// filepath.Join - joins rootpath + command - chrootDir + command (/tmp/mydocker-jail + /usr/local/bin/docker-explorer)
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
	} else if err != nil {
		fmt.Fprint(os.Stderr, "Failed to check if binary exists in image: %v\n", err)
		os.Exit(1)
	}

	// Make directory chroot
	if err := syscall.Chroot(chrootDir); err != nil {
		fmt.Fprint(os.Stderr, "Chroot failed: %v\n", err)
		os.Exit(1)
	}

	// Change current directory (path) to '/'
	if err := os.Chdir("/"); err != nil {
		fmt.Fprintf(os.Stderr, "Chdir failed: %v\n", err)
		os.Exit(1)
	}

	// Prepare cmd struct - pipe standard FD
	cmd := exec.Command(command, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin

	// Create PID namespace and unified time shared namespace (host)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID,
	}


	// Run the command - fork + execvp
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}

}


// Get manifest struct
func getImageManifest(imageName, imageVersion, token string) (Manifest, error) {

	digest, err := getManifestUrl(imageName, imageVersion, token)
	if err != nil {
		return Manifest{}, err
	}

	manifestURL := fmt.Sprintf(
		"https://registry.hub.docker.com/v2/library/%s/manifests/%s",
		imageName,
		digest,
	)

	req, err := http.NewRequest("GET", manifestURL, nil)
	if err != nil {
		return Manifest{}, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return Manifest{}, err
	}
	defer resp.Body.Close()

	var manifest Manifest
	err = json.NewDecoder(resp.Body).Decode(&manifest)
	if err != nil {
		return Manifest{}, err
	}

	return manifest, nil
}

// Get manifest url from returned manifest list, based on system architecture and OS
func getManifestUrl(imageName, imageVersion, token string) (string, error) {

	systemOS, systemArch := runtime.GOOS, runtime.GOARCH
	manifestURL := fmt.Sprintf(
		"https://registry.hub.docker.com/v2/library/%s/manifests/%s",
		imageName,
		imageVersion,
	)

	req, err := http.NewRequest("GET", manifestURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.list.v2+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to fetch manifest list (%d): %s", resp.StatusCode, string(body))
	}

	var manifestList ManifestList
	err = json.NewDecoder(resp.Body).Decode(&manifestList)
	if err != nil {
		return "", err
	}

	for _, manifest := range manifestList.Manifests {
		if manifest.Platform.Architecture == systemArch && manifest.Platform.OS == systemOS {
			return manifest.Digest, nil
		}
	}

	return "", fmt.Errorf("Manifest not found")

}

// Get authentication token for image:pull
func getAuthToken(imageName string) (string, error) {
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

// Parse imageString in order to return name and version of the image (ubuntu:latest) -> ubuntu, latest
func getImageNameAndVersion(imageString string) (string, string, error) {
	if !strings.Contains(imageString, ":") {
		return imageString, "latest", nil
	}
	parts := strings.Split(imageString, ":")
	imageName, imageVersion := parts[0], parts[1]

	return imageName, imageVersion, nil
}

// Fetch and extract into chroot directory all image layers
func getAllLayers(manifest Manifest, imageName, token, jailPath string) error {
	client := &http.Client{}

	for _, layer := range manifest.Layers {
		layerURL := fmt.Sprintf(
			"https://registry.hub.docker.com/v2/library/%s/blobs/%s",
			imageName,
			layer.Digest,
		)
		req, err := http.NewRequest("GET", layerURL, nil)
		if err != nil {
			return err
		}

		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(req)
		if err != nil {
			return err
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to download layer: %s (%d): %s", layer.Digest, resp.StatusCode, string(body))
		}

		err = extractTarGz(resp.Body, jailPath)
		if err != nil {
			return fmt.Errorf("error extracting layer %s: %v", layer.Digest, err)
		}
	}

	return nil
}

// Each image layer is .Tar file that has to be extracted - set chmod for every file
func extractTarGz(gzipStream io.Reader, targetDir string) error {
	gzReader, err := gzip.NewReader(gzipStream)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %v", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading tar: %v", err)
		}

		targetPath := filepath.Join(targetDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, header.FileInfo().Mode()); err != nil {
				panic("failed to mkdir: " + err.Error())
			}
		case tar.TypeReg:
			file, err := os.Create(targetPath)
			if err != nil {
				panic("failed to create: " + err.Error())
			}
			if _, err := io.Copy(file, tarReader); err != nil {
				panic("failed to copy: " + err.Error())
			}
			err = file.Close()
			if err != nil {
				panic("failed to close: " + err.Error())
			}

		case tar.TypeSymlink:
			absolutePath := filepath.Join(targetDir, header.Linkname)
			relativePath, err := filepath.Rel(filepath.Dir(targetPath), absolutePath)
			if err != nil {
				panic("failed relative: " + err.Error())
			}
			err = os.Symlink(relativePath, targetPath)
			if err != nil {
				panic("failed symlink: " + err.Error())
			}
		default:
			// Other types can be added here
		}

		err = os.Chmod(targetPath, header.FileInfo().Mode())
		if err != nil {
			if !os.IsNotExist(err) {
				panic("failed to chmod: " + err.Error())
			}
		}
	}

	return nil
}

// Copy file from src path to dst path
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
