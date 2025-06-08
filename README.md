# Mini Docker

## Overview
This document describes the **Mini Docker** project, a simplified, educational implementation of Docker designed to demystify its inner workings.  I’ve always been curious about how Docker really works under the hood, and what a better way to learn than by building it ourselves? Specifically, this project recreates the `docker run` command, focusing on executing commands in an isolated environment using Linux namespaces, and chroot jails. Below are the details of the project structure, challenges, and key concepts.

## Project Structure
The project consists of the following files:
- **`app/main.go`**: The core of the **Mini Docker**, a Go program that implements all logic for the project.
- **`your_docker.sh`**: A bash script that compiles the Go code and passes the provided command and arguments to the executable. - it replaces running `go build app/*.go` everytime
- **`Dockerfile`**: Sets up a test environment with the `golang:alpine` base image, installs `curl`, downloads `docker-explorer` - program used only for testing purposes, and configures the entrypoint to run `your_docker.sh`.

## Setup and Running the Application

To run the **Mini Docker** application, you need to set up the environment and dependencies correctly. Since the core concepts of Docker rely on Linux kernel features (e.g., namespaces, cgroups, chroot), a Linux environment is required. Below are the steps to set up and run the application, tailored for both Linux and non-Linux (Windows/macOS) users.

### Prerequisites
- **Linux Operating System**: The application uses Linux-specific syscalls (e.g., `CLONE_NEWPID`, `CLONE_NEWUTS`, `chroot`) that require a Linux kernel.
- **Go Compiler**: Needed to compile the `app/main.go` file, as the `your_docker.sh` script builds the Go program.
- **Git**: Required to clone the project repository.
- **Docker (for non-Linux users)**: If you're using Windows or macOS, Docker is required to provide a Linux environment with a Go compiler. Docker is used solely to create a Linux-based test environment, not to run the application logic itself.

### Steps to Set Up
1. **Clone the Repository**:
   ```bash
   git clone https://github.com/obradovicsl/mini-docker
   ```

2. **Navigate to the Project Directory**:
   ```bash
   cd mini-docker
   ```

### Running on Linux
If you're using a Linux system, you can run the application directly using the `your_docker.sh` script.

1. **Make the Script Executable**:
   ```bash
   chmod +x your_docker.sh
   ```

2. **Run the Application**:
   Use the following command format:
   ```bash
   ./your_docker.sh run <image> <command> <arg1> <arg2> ...
   ```
   Example:
   ```bash
   ./your_docker.sh run alpine:latest /bin/echo hello
   ```
   - **Expected Output**: `hello`

### Running on Windows/macOS
For non-Linux systems, you need to use Docker to create a Linux environment with the Go compiler.

1. **Create a Shell Alias for Convenience**:
   To simplify the process of building and running the Docker container, create an alias:
   ```bash
   alias mydocker='docker build -t mydocker . && docker run --cap-add="SYS_ADMIN" mydocker'
   ```
   - The `--cap-add="SYS_ADMIN"` flag is required to allow Linux syscalls like `chroot` and namespace creation.

2. **Run the Application**:
   Use the alias with the following command format:
   ```bash
   mydocker run <image> <command> <arg1> <arg2> ...
   ```
   Example:
   ```bash
   mydocker run alpine:latest /bin/echo hello
   ```
   - **Expected Output**: `hello`

### Notes
- **Why Docker on Windows/macOS?** Docker provides a Linux kernel environment (via WSL 2 on Windows or a lightweight Linux VM on macOS) necessary for the application's Linux-specific syscalls. The project itself is a simplified Docker implementation, but we use Docker only to create the test environment with a Linux kernel and Go compiler.
- **File Permissions**: Ensure `your_docker.sh` has executable permissions (`chmod +x your_docker.sh`) before running.

If you encounter issues, verify that:
- The Go compiler is installed (for Linux) or included in the Docker image.
- The correct image and command are specified (e.g., `alpine:latest` and `/bin/echo`).

---

## Challenge 01 - Execute a Command
- **Goal**: Execute a simple command provided via the command line using Go.
- **Overview**: 
- **Implementation**:
  - The `your_docker.sh` script compiles the `app/main.go` program and passes the provided command and arguments to the executable.
  - Example command:
    ```bash
    ./your_docker.sh run alpine:latest /path/to/your/executable/binary arguments
    ```
  - In `app/main.go`, the program extracts the command and its arguments from `os.Args` then uses `exec.Command(...)` to configure a process that will run the specified executable.
- To understand better how this works, you should be avare of fork and exec sys_calls: [Fork vs Exec](./concepts/fork-vs-exec.md)

## Challenge 02: Pipe Stdout and Stderr
- **Goal**: Pipe the child process's stdout and stderr to the parent process.
- **Implementation**:
  - Modified `main.go` to set:
    ```go
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Stdin = os.Stdin
    ```
- More on file descriptors: [File descriptors](./concepts/file-descriptors.md)


## Challenge 03: Handle Exit Codes
- **Goal**: Relay the child process's exit code to the parent process.
- **Implementation**:
  - Check for `*exec.ExitError` to retrieve the exit code:
    ```go
    if err := cmd.Run(); err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            os.Exit(exitErr.ExitCode())
        }
        os.Exit(1)
    }
    ```
  - **Explanation**:
    - The `exec` package's `ExitError` struct embeds `os.ProcessState`, which provides the `ExitCode()` method
    - This ensures the parent process exits with the same code as the child.
- To fully understand this behavior, it's helpful to know how Go interfaces work and how they support polymorphism. 👉 [Go Interfaces](./concepts/go-interfaces.md)

## Challenge 04: Filesystem Isolation
- **Goal**: Restrict the child process's filesystem access using `chroot`.
- **Implementation**:
  - Create a temporary chroot jail directory using `os.MkdirTemp`:
    ```go
    chrootDir, err := os.MkdirTemp("", "mydocker-jail")
    ```
  - Copy the executable to the jail:
    ```go
    destPath := filepath.Join(chrootDir, command)
    os.MkdirAll(filepath.Dir(destPath), 0755)
    copyFile(command, destPath)
    ```
  - Call `syscall.Chroot` and set the working directory:
    ```go
    syscall.Chroot(chrootDir)
    os.Chdir("/")
    ```
  - **Issue**: `Cmd.Run()` requires `/dev/null`. Options:
    1. Create `/dev/null` in the chroot jail:
       ```go
       syscall.Mknod(filepath.Join(chrootDir, "dev/null"), syscall.S_IFCHR|0666, 0)
       ```
    2. Ensure `Cmd.Stdout`, `Cmd.Stderr`, and `Cmd.Stdin` are not `nil`.

## Challenge 05: Process Isolation
- **Goal**: Isolate the process tree using PID namespaces.
- **Implementation**:
  - Use `SysProcAttr` with `CLONE_NEWPID`:
    ```go
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID,
    }
    ```
  - **Details**:
    - `CLONE_NEWPID` creates a new PID namespace, making the child process appear as PID 1.
    - `CLONE_NEWUTS` isolates the hostname.
    - Requires Linux-specific syscalls, hence the `//go:build linux` directive.

## Challenge 06: Fetch Docker Image
- **Goal**: Fetch a Docker image from the Docker Registry and extract it into the chroot jail.
- **Implementation**:
  1. **Authenticate**:
     - Send an HTTP GET request to:
       ```text
       https://auth.docker.io/token?service=registry.docker.io&scope=repository:library/<image>:pull
       ```
     - Returns a bearer token.
  2. **Get Manifest**:
     - Request the manifest list:
       ```text
       https://registry.hub.docker.com/v2/library/<image>/manifests/<tag>
       ```
       - Headers:
         ```text
         Authorization: Bearer <token>
         Accept: application/vnd.docker.distribution.manifest.list.v2+json
         ```
     - Select the manifest for the current architecture (`runtime.GOOS` and `runtime.GOARCH`).
     - Request the image manifest:
       ```text
       https://registry.hub.docker.com/v2/library/<image>/manifests/<digest>
       ```
       - Headers:
         ```text
         Accept: application/vnd.docker.distribution.manifest.v2+json
         ```
  3. **Fetch and Extract Layers**:
     - Download each layer (`.tar.gz` files) from:
       ```text
       https://registry.hub.docker.com/v2/library/<image>/blobs/<digest>
       ```
     - Extract layers into the chroot jail using a function like `extractTarGz`.
     - Combine layers in order to reconstruct the image filesystem.

#### Notes on Cross-Platform Execution
- **Question**: How do Linux binaries in Docker images run on Windows or macOS?
- **Answer**:
  - Docker images contain Linux ELF executables, compiled for Linux environments.
  - On **Windows**, Docker uses WSL 2, which provides a Linux kernel.
  - On **macOS**, Docker runs containers in a lightweight Linux VM using Apple's Hypervisor framework.
  - The Linux kernel ensures compatibility, regardless of the host OS.

---

## Technical Details

### Linux Namespaces
- **PID Namespace**: Isolates the process tree, making the container's process appear as PID 1.
  - Enabled via `CLONE_NEWPID`.
- **UTS Namespace**: Isolates the hostname and domain name.
  - Enabled via `CLONE_NEWUTS`.

### CGroups
- Limit resources like memory, CPU, and I/O.
- Not explicitly implemented in the provided code but can be added using Linux cgroups APIs.

### Chroot
- Changes the root directory for a process and its children, creating a "jail" that restricts filesystem access.
- Implemented using:
  ```go
  syscall.Chroot(chrootDir)
  os.Chdir("/")
  ```

### Docker Registry API
- **Authentication**: Uses OAuth2 bearer tokens.
- **Manifest List**: Provides a list of manifests for different platforms.
- **Manifest**: Contains the list of filesystem layers.
- **Layers**: `.tar.gz` files that form the image filesystem when extracted.

---

## Next Steps
- Enhance error handling in `main.go` for HTTP requests and system calls.
- Add support for additional namespace types (e.g., network namespaces).
- Implement cgroup resource limits for memory and CPU.
- Improve the `extractTarGz` function to handle more tar file types (e.g., hard links).