# Interfaces in Go

### 🔍 What Are Interfaces?

In Go, interfaces are a way to define behavior. You don’t care what the concrete type is — you just care about what it **can do**. That’s why we say interfaces are about **can-do semantics**.

An interface is a set of method signatures. If a type defines all the methods that an interface requires — it **automatically** implements that interface.

There’s no need for explicit declaration like `implements` in Java or C#. This implicitness is both powerful and a bit weird at first.

---

## 🐶 Example: Dog Can Speak

```go
// Define an interface
type Speaker interface {
    Speak() string
}

// Two concrete types
type Dog struct{}
type Cat struct{}

// Method for Dog (not a plain function — a method on Dog)
func (d Dog) Speak() string {
    return "Woof"
}

// This is just a plain function — not related to interface
func Speak() string {
    return "Speaking"
}

func main() {
    var s Speaker

    // Dog implements Speaker (has Speak method)
    s = Dog{}
    fmt.Println(s.Speak()) // OK

    // Cat doesn't implement Speak() => this will fail to compile
    s = Cat{} // ❌ compile-time error
    fmt.Println(s.Speak())
}
```

Only types that define **all methods** required by an interface implement it — **automatically**.

---

## ⚠️ Common Confusion

You might think:

> "Well, Dog and Cat are both structs — so shouldn’t they both implement the interface?"

Not unless they define the **required methods**. Interfaces care about what methods exist — not the names of the types.

---

## 💥 The `error` Interface

This is a built-in interface you use all the time — even if you didn’t realize it:

```go
type error interface {
    Error() string
}
```

Any type that defines an `Error() string` method automatically satisfies the `error` interface.

This is how `fmt.Errorf`, `errors.New`, and even custom error structs work under the hood.

---

## 🎯 Type Assertion: Checking Interface Implementation

Sometimes, you’ll want to check if a value actually **is** a specific type behind the interface — this is called a **type assertion**:

```go
type ExitError struct {
    *os.ProcessState
    Stderr []byte
}

func (e *ExitError) Error() string {
    return e.ProcessState.String()
}

func main() {
    var err error = &ExitError{}

    if exitErr, ok := err.(*ExitError); ok {
        // Now we can access exitErr.Stderr, etc.
        fmt.Println("Got an ExitError")
    }
}
```

Here, we’re saying: "Hey, this thing that implements `error` — is it actually our `ExitError`?"

This kind of check is crucial when you’re handling errors returned from commands, like in:

```go
if err := cmd.Run(); err != nil {
    if exitErr, ok := err.(*exec.ExitError); ok {
        os.Exit(exitErr.ExitCode())
    }
    os.Exit(1)
}
```

---

## 🧠 TL;DR

* Interfaces in Go are **implicitly implemented** — if it walks like a duck and quacks, it’s a duck.
* You can assign a value of any type to an interface as long as it satisfies the interface.
* Interfaces enable polymorphism and clean abstractions.
* Built-in interfaces like `error` are everywhere — and they work because of this model.
* Use **type assertions** when you want to access the actual value behind the interface.
