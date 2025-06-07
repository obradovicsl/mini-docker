  - The call to `cmd.Run()` in Go triggers the classic **fork-exec** mechanism used by UNIX-like operating systems:
    - First, a **child process is created** — similar to `fork()` in C
    - Then, within the child process, the `execve(...)` system call is invoked to replace the process image with the specified binary (e.g., `/usr/local/bin/docker-explorer`).
  - In simpler terms, your program doesn't directly "run a command" — it **spawns a new process** and then **replaces** it with the target executable.
  - The Go standard library wraps all this logic in a clean, high-level API (`exec.Cmd.Run()`), but under the hood, it's relying on **system calls**.


  ### Why This Matters
  Understanding how fork() and exec() work isn’t just trivia — it’s foundational when you're trying to build something like a container from scratch. Even though Go makes it look clean, at the system level it’s still doing what Linux has always done. 
  And that’s the beauty of it: using a high-level language like Go, You get to orchestrate all this low-level process control, without writing raw syscalls by yourself.