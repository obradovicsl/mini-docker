# Fork vs Exec

### How Go Actually Runs a Command

When we write something like:

```go
cmd := exec.Command("/usr/local/bin/docker-explorer", "echo", "hey")
cmd.Run()
```
... it looks like we're just running a command.

But under the hood, Go is triggering the classic **fork-exec** mechanism that every UNIX-like OS relies on:
- First, a **child process is created** — similar to `fork()` in C
- Then, within the child process, the `execve(...)` system call is invoked to replace the process image with the specified binary (e.g., `/usr/local/bin/docker-explorer`).
- The result: the child process is no longer running your Go code — it's now running the command you wanted to execute.
- In simpler terms, your program doesn't directly "run a command" — it **spawns a new process** and then **replaces** it with the target executable.
- The Go standard library wraps all this logic in a clean, high-level API (`exec.Cmd.Run()`), but under the hood, it's relying on **system calls**.


### Why This Matters
Understanding how `fork()` and `exec()` work isn’t just trivia — it’s foundational whenyou're trying to build something like a container from scratch. 

**fork ➝ exec**

Even though Go abstracts it nicely, it’s still the Linux kernel doing the real work underneath. And that’s kind of the magic — you're orchestrating low-level syscalls, but you're writing clean, readable Go code.

You don’t have to write `syscall.ForkExec()` by hand to understand what’s happening.

But when something doesn’t work as expected — knowing this stuff makes debugging 100x easier.