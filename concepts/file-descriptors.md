# File Descriptors: Stdout and Stderr Plumbing

### What Happens When You Set `cmd.Stdout = os.Stdout`


When we set this in Go:
```go
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr
```
…it might seem like we're just telling Go where to send the output.

But under the hood, something very Unix-y is happening:

We're **reusing the parent process’s file descriptors** — specifically:
- `0` for stdin
- `1` for stdout
- `2` for stderr

The child process will inherit these file descriptors when it's created (via `fork()`), and then it will use them as if they were its own.
So when the child writes to its stdout, it’s actually writing to our terminal — because we gave it our file descriptor.

This is how output flows from the child to the screen without needing any extra piping code.


### Why This Matters

>This wiring makes it look like the child is "printing to the terminal," but really, it's just writing to a number — a file descriptor — that happens to point to the same thing the parent was writing to.

No magic. Just file descriptors.

It’s also why tools like Docker or bash can run a subprocess and see its output immediately — they pass the same FDs down the chain.