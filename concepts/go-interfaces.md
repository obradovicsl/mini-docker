#### Go Interfaces and Polymorphism
- **Interfaces in Go**:
  - Interfaces define a set of methods that a type must implement.
  - Go implements interfaces implicitly: any type with the required methods automatically implements the interface.
  - Example:
    ```go
    type Speaker interface {
        Speak() string
    }
    type Dog struct{}
    type Cat struct{}
    func (d Dog) Speak() string { return "Woof" }
    func (c Cat) Speak() string { return "Meow" }
    ```
    - Both `Dog` and `Cat` implement `Speaker` because they define `Speak()`.
  - **Error Interface**:
    - Defined as:
      ```go
      type error interface {
          Error() string
      }
      ```
    - Any type with an `Error() string` method implements the `error` interface.
    - `exec.ExitError` implements `error` and embeds `os.ProcessState`, allowing access to `ExitCode()`.