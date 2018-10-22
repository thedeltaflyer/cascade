# Go Cascade

Cascade is a system for assisting with goroutine lifecycles in a top-down fashion.

This is a tool for organizing complex Golang applications that have routines that are reliant on children that need to be exited/killed in a particular order in order to safely clean up and exit.

A Cascade can be thought of as a tree where one parent can have many children and the children must exit before the parent. Helpers for keeping track of functions/goroutines are included as well as a system for running cleanup actions in a safe order.

For the full documentation, see the [godocs](https://godoc.org/github.com/thedeltaflyer/cascade).

## Basic Usage:

### Root and Child Cascades:
Create a root Cascade:
```go
rootCas := cascade.RootCascade()
```

Create a child from a root Cascade:
```go
child1 := rootCas.ChildCascade()
```

Create a child from a child Cascade:
```go
child2 := child1.ChildCascade()
```

### Marking a function to be monitored by a Cascade
Note: A function that is marked by a Cascade must un-mark when it completes otherwise the Cascade could get stuck while trying to kill
```go
rootCas := cascade.RootCascade()
go func markedFunc() {
	rootCas.Mark() // This function is now marked!
	// Do things, maybe block until killed?
	rootCas.UnMark() // This function is done, let the mark go!
}()
go func markedFuncWithDefer() {
	rootCas.Mark() // This function is now marked!
	defer rootCas.UnMark() // We'll un-mark when this function is done!
	// Do stuff
	return
}()
```

### Blocking until Killed:
Block execution until a Cascade is Killed:
```go
rootCas := cascade.RootCascade()
go func asyncFunc() {
	rootCas.Mark()
	// Do some setup
	rootCas.Hold() // Hold here until the Kill command is executed
	// Do cleanup
	rootCas.UnMark()
}
```
Or in a loop:
```go
rootCas := cascade.RootCascade()
go func asyncFunc() {
	rootCas.Mark()
	defer rootCas.UnMark()
	// Do some setup
	for {
		select {
		case <-rootCas.Dying():
			return
		default:
			continue
		}
	}
}
```

### Kill functions:
To Kill the current Cascade and any child Cascades that have been created by it:
```go
child1.Kill()  // This will kill everything managed by child1 and it's child, child2
```

To Kill everything from root on if you're not on the root Cascade or if you're not sure if it's the root Cascade:
```go
child1.KillAll()  // This will kill everything managed by this Cascade tree: rootCas, child1, and child2
```

### Cancel functions:
To Kill the current Cascade and any child Cascades that have been created by it without performing extra actions:
```go
child1.Cancel()  // This will kill everything managed by child1 and it's child, child2
```

To Cancel everything from root on if you're not on the root Cascade or if you're not sure if it's the root Cascade:
```go
child1.CancelAll()  // This will kill everything managed by this Cascade tree: rootCas, child1, and child2
```

### Wait until Kill is complete:
Warning: Do not add a wait within a Cascade-managed function as it could cause the exit of the Cascade to be blocked!

```go
rootCas := cascade.RootCascade()
func sendKill() {
	rootCas.Kill()     // Send the kill command
	rootCas.Wait()     // Wait until all managed functions have exited
	rootCas.WaitDone() // Wait until all managed functions have exited AND any other actions have been executed
}
```

### Check if a Cascade has already been Killed
```go
var d bool = rootCas.IsDead()  // true if killed, false if alive
var a bool = rootCas.Alive()   // true if alive, false if killed
```

### Things to do on Kill
A Cascade can run a number of actions before completing the kill command, such as cleanup tasks or logs.

Note: Cancels will bypass any queued actions!

```go
rootCas := cascade.RootCascade()

myAction1 := func() {
	fmt.Println("Action 1!")
}

myAction2 := func() {
	fmt.Println("Action 2!")
}

myAction3 := func() {
	fmt.Println("Action 3!")
}

rootCas.DoOnKill(myAction1)
rootCas.DoOnKill(myAction2)
rootCas.DoFirstOnKill(myAction3)

rootCas.Kill()
```
Output:
```
Action 3!
Action 1!
Action 2!
```

Actions on a Cascade will run *after* all the of actions of its children have run:
```go
rootCas := cascade.RootCascade()
child1 := rootCas.ChildCascade()

myAction1 := func() {
	fmt.Println("Action 1!")
}

myAction2 := func() {
	fmt.Println("Action 2!")
}

myAction3 := func() {
	fmt.Println("Action 3!")
}

rootCas.DoOnKill(myAction1)
child1.DoOnKill(myAction2)
rootCas.DoFirstOnKill(myAction3)

rootCas.Kill()
```
Output:
```
Action 2!
Action 3!
Action 1!
```

### Note on Killing and Garbage Collection
You *MUST* kill every Cascade you create to ensure that it does not get left in memory. A parent cascade keeps a record of children that will keep the garbage collector from removing them. Killing a child will remove its reference from its parent.

### Further Reading
See the [godocs](https://godoc.org/github.com/thedeltaflyer/cascade) for a full list of all available functions!