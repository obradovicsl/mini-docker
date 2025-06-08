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

## Overview: What Does `docker run` *Really* Do?
At its core, a Docker container is nothing magical — it's just a process. A regular old Linux process. It has a PID, a user, a group, you can `kill` it with signal 9, and you can see it with `ps aux`. No bleeding-edge tech — just clever use of old kernel primitives.
But when you run:
```bash
docker run alpine:latest sh
```
a lot of things happen under the hood. Here's what it roughly boils down to:
1. **A new process is created.**
 - This will be the actual thing(executable/program) you want to run — for example, `/bin/sh`. Under the hood, this is done by forking the current process, and then calling `execvp` to turn that child into the command you passed.
2. **Standard I/O is wired.**
 - We hook up the child's stdin, stdout, and stderr to the parent process, so we can interact with the container like a normal CLI tool.
3. **But it's not isolated yet.**
 - At this point, we’ve just launched a new process — but it can still see the full host filesystem. It can `ls /home`, read your secrets, modify files, and even see and interact with all the other processes on the system. Not good.
4. **So we isolate the filesystem using `chroot`.**
 - We set up a “jail” directory, and use `chroot` to change the root of the child process. From its perspective, it’s now running in a new root — it literally cannot see anything outside that directory.
5. **We isolate the process tree using PID namespaces.**
 - The container gets its own PID namespace. From inside, it sees itself as PID 1 — like an init system. It has no idea any other processes even exist. It can’t send signals outside its namespace. It lives in its own little world.
6. **But wait — the jail is empty.**
 - Running ls inside the jail shows... nothing. A blank filesystem. That’s useless.
7. **We download and unpack the container image.**
 - Using the Docker Registry HTTP API, we fetch something like `alpine:latest`, unpack it, and lay it out inside the jail directory. Now, our process sees a complete, working Linux root filesystem, with basic tools and libraries preinstalled.

---

## Challenge 01 - Execute a Command
- **Goal**: Execute a simple command provided via the command line using Go.
- **Overview**: Here, we're mimicking the basic behavior of `docker run`. The goal is simple: take a command and its arguments from the user, and use Go to run that binary as a child process. No isolation yet — just launching a program based on input, like Docker does behind the scenes.
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
- **Overview** In this step, we want the child process to behave like it's running directly in your terminal — so its output and input go through your screen and keyboard. We connect its standard input/output/error streams to match the parent’s, just like real containers do.
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
- **Overview**: Here we make sure that if the child process fails or exits with a specific code, our program does the same. This is important so that other tools or scripts using our program can know what actually happened — just like with real CLI tools.
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

# Challenge 04: Filesystem Isolation

### What is chroot?

`chroot` (short for *change root*) is a system call that changes the root directory (`/`) for the current process and all of its children. Once the process is inside the chroot environment, it can't see or access files outside of that new root — effectively isolating its view of the filesystem. This isolated view is often called a **chroot jail**.

### What We’re Building

We’ll set up a basic chroot jail for the child process that gets spawned during `mydocker run ...`. This involves:

1. Creating a temporary jail directory.
2. Copying the binary being executed into the jail.
3. Calling `chroot()` before executing the child process.
4. Handling quirks like needing a valid `/dev/null`.

## Implementation Steps

### 1. Create a chroot jail directory

We use Go's `os.MkdirTemp` to create an isolated temporary directory:

```go
chrootDir, err := os.MkdirTemp("", "mydocker-jail")
```

This gives us a unique path like `/tmp/mydocker-jail-123456`, where we'll simulate the root.

### 2. Copy the executable to the jail

We need to ensure the binary we want to run (e.g., `/usr/local/bin/docker-explorer`) exists inside the jail. This requires:

* Building the full destination path inside the jail:

  ```go
  destPath := filepath.Join(chrootDir, command)
  ```
* Making sure all parent directories exist:

  ```go
  os.MkdirAll(filepath.Dir(destPath), 0755)
  ```
* Copying the binary from the host into the jail:

  ```go
  copyFile(command, destPath)
  ```

### 3. Enter the jail using chroot

Once the setup is complete:

```go
syscall.Chroot(chrootDir)
os.Chdir("/")
```

This sets `/` from the child process's perspective to be the root of the chroot jail.

### 4. Handle /dev/null issue

The Go `exec.Cmd` family expects `/dev/null` to exist by default. If it's not there, things may break.
You have two options:

#### Option 1: Create /dev/null manually

```go
syscall.Mkdir(filepath.Join(chrootDir, "dev"), 0755)
syscall.Mknod(filepath.Join(chrootDir, "dev/null"), syscall.S_IFCHR|0666, int(unix.Mkdev(1, 3)))
```

#### Option 2: Manually wire `os.Stdin`, `os.Stdout`, and `os.Stderr`

If these are explicitly set (and not `nil`), Go won’t complain about `/dev/null` missing:

```go
cmd.Stdin = os.Stdin
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr
```


## Summary

With chroot in place, we’ve created a lightweight version of filesystem isolation — a key building block of real containers. It's not perfect security, but it’s a fundamental concept worth learning. Just like Docker isolates containers, here we’re isolating our process to only see what we allow it to see.


# Challenge 05: Process Isolation

### Goal

Isolate the process tree using PID namespaces so that the child process does not have access to other processes on the host system.

### Why This Matters

Even though we already limited filesystem access using `chroot`, a malicious or misbehaving program could still interact with other processes. It could list them, send them signals, or monitor what they're doing. To prevent that, we want to isolate the **process namespace**, so that the program only sees itself and its children — just like in a real container.

### Overview

In Linux, this type of isolation is made possible with **namespaces**. Namespaces wrap global system resources and make them appear private to processes inside the namespace. There are many types of namespaces (network, mount, user, etc.), and here we focus on:

* `CLONE_NEWPID`: to isolate the process tree
* `CLONE_NEWUTS`: to isolate the hostname

We’ll apply these using Go’s `SysProcAttr`, which is part of the `os/exec` package. But keep in mind — these are Linux-specific features, so this solution won’t compile on Windows or macOS.

### Implementation

### Step 1: Enable Linux-specific build

At the top of your `main.go`, add:

```go
//go:build linux
// +build linux
```

This ensures the file only compiles when building on a Linux system.
If you don’t add this, and try to build or run the code on a non-Linux machine (like macOS or Windows), you’ll get compilation errors. That’s because:

 - Linux-specific system calls like `syscall.Clone` and constants like `CLONE_NEWPID` or `Cloneflags` don’t exist on other platforms.
 - These features rely on Linux kernel namespaces, which are not available on other operating systems.
By using the `//go:build linux directive`, you’re telling the Go compiler:

“Only include this file when building for Linux.”
This prevents the program from breaking on incompatible systems.

### Step 2: Set `SysProcAttr`

In your `main.go`, when you configure the command to run, set the `SysProcAttr` field:

```go
cmd.SysProcAttr = &syscall.SysProcAttr{
    Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID,
}
```

### Step 3: Why It Works

* `CLONE_NEWPID` tells the kernel to create a new PID namespace.

  * Inside that namespace, the child process will see itself as PID 1.
  * It will not be able to see or signal any process outside of the namespace.
* `CLONE_NEWUTS` creates a new UTS (Unix Time-Sharing) namespace.

  * It allows the child process to have its own hostname, isolated from the host system.

### Step 4: Caveats

* You must launch a child **shell** process for the new PID namespace to take effect properly — running a direct command won’t create a new PID namespace properly unless the process forks another child inside it.
* All of this depends on Linux’s `clone()` system call, which is a lower-level variant of `fork()` that allows for precise namespace control.

### Additional Resources

* Learn more about `/proc` and PID 1 behavior in containers
* Dive deeper into Linux namespaces in man pages: `man 7 namespaces`

---

This challenge builds a critical part of containerization — **process isolation** — and gives us another piece of what makes tools like Docker so powerful, but also so elegantly built on top of the Linux kernel.


# Challenge 06: Fetch Docker Image

### Goal

Fetch a Docker image from the Docker Registry and extract it into our chroot jail so the container has a usable filesystem.

### Overview
At this point, we've isolated the filesystem (using `chroot`) and the process tree (using `PID namespaces`). However, from the process's perspective, the current root directory is still empty — if we ran ls, we'd see nothing inside. What we're missing is the actual content of a container image — a functional Linux root filesystem with tools and libraries (e.g., from `alpine:latest`). In this challenge, we'll use the Docker Registry HTTP API to download and unpack such an image into our jail directory, so that the process sees a complete root filesystem when it starts.

We'll perform the same steps that a real Docker client does under the hood:

1. Authenticate with the Docker Registry.
2. Fetch the manifest list for the image.
3. Select the manifest that matches our system's OS and architecture.
4. Download each layer.
5. Extract all the layers in order into the chroot jail.

---

### Step-by-step

#### 1. Authenticate with the Docker Registry

To access any image, we must first obtain an access token. This is done by sending a GET request to:

```
https://auth.docker.io/token?service=registry.docker.io&scope=repository:library/<image>:pull
```

If the image name is valid, the server will return a Bearer token. We'll use this token to authenticate our next requests.

---

#### 2. Get the Manifest List

A manifest is essentially a blueprint of an image — it lists all the filesystem layers the image is made of.

In newer versions of the Docker Registry API, the initial manifest request returns a **manifest list** — a list of manifests for each platform (OS and architecture).

We send a GET request with the proper headers:

```
GET https://registry.hub.docker.com/v2/library/<image>/manifests/<tag>

Headers:
Authorization: Bearer <token>
Accept: application/vnd.docker.distribution.manifest.list.v2+json
```

This response includes multiple manifests. **We must choose the one that matches our host OS and CPU architecture** — for example, x86_64 or ARM. In Go, we can determine this using the built-in `runtime.GOOS` and `runtime.GOARCH` constants.

---

#### 3. Get the Platform-Specific Manifest

Once we’ve selected the correct manifest from the list, we send another GET request to fetch it:

```
GET https://registry.hub.docker.com/v2/library/<image>/manifests/<digest>

Headers:
Authorization: Bearer <token>
Accept: application/vnd.docker.distribution.manifest.v2+json
```

This response will contain a list of layers, each represented by a digest (hash).

---

#### 4. Download and Extract Layers

Each layer is a `.tar.gz` archive that contains part of the image's file system.

To download a layer:

```
GET https://registry.hub.docker.com/v2/library/<image>/blobs/<digest>

Headers:
Authorization: Bearer <token>
```

After downloading each layer, we extract it using a Go function like `extractTarGz()`. **Order is important** — layers are applied sequentially to reconstruct the full filesystem.

**Important notes:**
- After extracting a layer, **we must ensure that any binary/executable files have the correct permissions**. This means calling `chmod` to make them executable if needed (e.g. `chmod +x <path>`).
- Additionally, **we must verify that the command we intend to run exists in the extracted filesystem**. If it doesn't, we should **manually copy it from the host filesystem into the chroot jail** before execution. This is crucial for minimal base images that may lack common tools.

---

### Summary

With this step, we're finally able to give our container a proper root filesystem by dynamically pulling and unpacking Docker images. This mimics what real container runtimes like Docker or containerd do, but we’re doing it ourselves using the public API.

**As a result:**
- Our chroot directory now contains the full filesystem specified by the provided Docker image.
- Inside this jail, we can use all the tools that come with the image — such as `echo`, `ls`, and other common Linux utilities.
- We have a functional, minimal container environment capable of running real Linux programs, isolated by both filesystem and PID namespaces.

---


#### Question: Aren’t the binaries inside a container image (like `bash`, `grep`, etc.) compiled for a specific Linux environment? How can they run on Windows or macOS host machine?

Yes — the binaries bundled in container images are Linux ELF executables, compiled to run on a Linux system with a Linux kernel. So how do they work on non-Linux hosts?

- **On Windows**, Docker relies on **WSL 2 (Windows Subsystem for Linux 2)**, which provides a full Linux kernel and environment. Containers actually run inside that Linux kernel.
- **On macOS**, Docker uses a **lightweight virtual machine** (managed through Apple’s Hypervisor.framework) to boot a minimal Linux OS and run containers inside it.

In both cases, even though the host OS is not Linux, the actual execution of container processes happens inside a real Linux environment. That’s how these Linux-compiled binaries are able to run correctly and consistently across platforms.


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

---

## Next Steps
- Add support for additional namespace types (e.g., network namespaces).
- Implement cgroup resource limits for memory and CPU.
- Improve the `extractTarGz` function to handle more tar file types (e.g., hard links).