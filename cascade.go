// Package cascade is a system for tracking and controlling a series of goroutines.
// It allows for the creation of tracked routines, conversion of non-blocking functions
// into tracked goroutines, and manually tracking goroutines.
//
// Cascade allows tying into various points of a lifecycle including "dying", "dead", and "done"
// states. Goroutines can be killed or cancelled at any point from any part of the program.
//
// The system allows for a parent-child branching similar to cancelable contexts except that it
// ensures that routines tracked by a child Cascade fully exit before the routines tracked
// by the parent. This means that there is a guarantee that a parent routine will not exit before
// all of its children have cleanly exited and proper cleanup has been done.
//
// You can also set actions to be run at each level at a guaranteed time and order.
// It is also possible to tie Cascades to Contexts or Contexts to Cascades to move between
// different systems.
package cascade

import (
	"context"
	"errors"
	"sync"
)

// Version is the current version of Cascade.
const Version string = "0.0.2"

// Cascade is the core structure of the cascade package. It contains all of the
// non-public resources used to maintain all tracked routines.
type Cascade struct {
	parent      *Cascade
	children    map[*Cascade]interface{}
	muChildren  sync.Mutex
	dying       chan interface{}
	onceDying   sync.Once
	dead        chan interface{}
	onceDead    sync.Once
	done        chan interface{}
	onceDone    sync.Once
	isDead      bool
	muDead      sync.RWMutex
	actions     []func()
	muActions   sync.Mutex
	onceActions sync.Once
	tracked     int
	muTracked   sync.RWMutex
	ctx         context.Context                    // A context that will Kill this Cascade
	trackedCtx  map[context.Context]trackedContext // Contexts that will be cancelled when this cascade gets Killed
	muCtx       sync.Mutex
	err         error
	muErr       sync.Mutex
}

// trackedContext struct manages any tracked Context items since we need to also track their "cancel" function.
type trackedContext struct {
	context context.Context
	cancel  func()
}

// RootCascade creates a new Cascade that is fully-initialized and ready to go.
// This Cascade also acts as the root cascade which means that it is the highest-up parent.
//
// Note about RootCascade and Errors
//
// When calling `KillAllWithError` or `CancelAllWithError`, the `RootCascade` Cascade is the only one that will
// receive the passed error.
func RootCascade() *Cascade {
	return &Cascade{
		nil,
		make(map[*Cascade]interface{}),
		sync.Mutex{},
		make(chan interface{}, 0),
		sync.Once{},
		make(chan interface{}, 0),
		sync.Once{},
		make(chan interface{}, 0),
		sync.Once{},
		false,
		sync.RWMutex{},
		make([]func(), 0),
		sync.Mutex{},
		sync.Once{},
		0,
		sync.RWMutex{},
		nil,
		make(map[context.Context]trackedContext, 0),
		sync.Mutex{},
		nil,
		sync.Mutex{},
	}
}

// Executes queued actions
func (c *Cascade) runActions() {
	c.onceActions.Do(func() {
		c.muActions.Lock()
		for _, action := range c.actions {
			action()
		}
		c.muActions.Unlock()
	})
}

func (c *Cascade) removeChild(child *Cascade) {
	c.muChildren.Lock()
	delete(c.children, child)
	c.muChildren.Unlock()
}

func (c *Cascade) cancelTrackedContexts() {
	c.muCtx.Lock()
	for _, tracked := range c.trackedCtx {
		tracked.cancel()
	}
	c.trackedCtx = nil
	c.muCtx.Unlock()
}

func (c *Cascade) closeAndClean(actions bool) {
	c.muChildren.Lock()
	c.children = nil
	c.muChildren.Unlock()
	c.onceDying.Do(func() {
		close(c.dying) // This Cascade is dying! bye bye
	})
	c.muTracked.RLock()
	if c.tracked == 0 {
		c.onceDead.Do(func() {
			close(c.dead)
		})
		c.muTracked.RUnlock()
	} else {
		c.muTracked.RUnlock()
		c.Wait()
	}
	if actions {
		c.runActions()
	}
	c.cancelTrackedContexts()
	if c.parent != nil {
		c.parent.removeChild(c)
	}
	c.onceDone.Do(func() {
		close(c.done) // This Cascade is done! bye bye
	})
}

// Wrap wraps a function that takes a Cascade as an argument and turns it into a tracked function.
//
// This is NOT a goroutine and will block until the provided function exits.
//
// The provided function MUST implement an exit condition using the provided Cascade.
//
// For an example of a suitable function, see the example for the `Go` function.
func (c *Cascade) Wrap(f func(*Cascade)) {
	c.Mark()
	defer c.UnMark()
	f(c)
}

// WrapInLoop wraps a function inside a loop and runs it as a tracked function.
//
// This is NOT a goroutine and will block until the provided function exits.
//
// The provided function MUST not block, it will continue getting called until the Cascade is killed or cancelled.
//
// Warning: The only way to exit the function is to kill or cancel the Cascade.
func (c *Cascade) WrapInLoop(f func()) {
	c.Mark()
	defer c.UnMark()
	for {
		select {
		case <-c.Dying():
			return
		default:
			f()
		}
	}
}

// WrapInLoopWithBool wraps a function inside a loop and runs it as a tracked function as long as the
// provided function returns `true`
//
// This is NOT a goroutine and will block until the provided function exits.
//
// The provided function MUST not block, it will continue getting called until the Cascade is killed or cancelled
// or the provided function returns `false`
func (c *Cascade) WrapInLoopWithBool(f func() bool) {
	c.Mark()
	defer c.UnMark()
	var fDone bool
	for {
		select {
		case <-c.Dying():
			return
		default:
			fDone = f()
			if !fDone {
				return
			}
		}
	}
}

// Go wraps a function that takes a Cascade as an argument and runs it as a tracked goroutine.
//
// The returned Cascade is a child of the current Cascade that is tracking the provided function.
//
// The provided function MUST implement an exit condition using the provided Cascade.
//
// Example Function:
//  func(c *Cascade) {
//  	// Do Something
//  	<-c.Dying() // Block until the exit condition
//  }
func (c *Cascade) Go(f func(*Cascade)) *Cascade {
	child := c.ChildCascade()
	go child.Wrap(f)
	return child
}

// GoInLoop wraps a function inside a loop and runs it as a tracked goroutine.
//
// The provided function MUST not block, it will continue getting called until the Cascade is killed or cancelled.
//
// The returned Cascade is a child of the current Cascade that is tracking the provided function.
//
// Warning: The only way to exit the function is to kill or cancel the Cascade.
func (c *Cascade) GoInLoop(f func()) *Cascade {
	child := c.ChildCascade()
	go child.WrapInLoop(f)
	return child
}

// GoInLoopWithBool wraps a function inside a loop and runs it as a tracked goroutine as long as the function
// returns `true`
//
// The provided function MUST not block, it will continue getting called until the Cascade is killed or cancelled
// or the provided function returns `false`
//
// The returned Cascade is a child of the current Cascade that is tracking the provided function.
func (c *Cascade) GoInLoopWithBool(f func() bool) *Cascade {
	child := c.ChildCascade()
	go child.WrapInLoopWithBool(f)
	return child
}

// Hold blocks until the Cascade is considered dying.
//
// This should be what goroutines use to determine when to exit.
func (c *Cascade) Hold() {
	<-c.dying
}

// Wait until the Cascade is considered dead.
//
// This can be used as a signal to indicate when all goroutines have exited.
// However, actions may not have run yet
func (c *Cascade) Wait() {
	<-c.dead
}

// WaitDone blocks until the Cascade is completely done.
//
// This can be used as a signal to indicate when all goroutines have exited and
// all actions have been completed.
func (c *Cascade) WaitDone() {
	<-c.done
}

// Dying provides a channel that will close once the Cascade is considered dying.
//
// This should be what goroutines use to determine when to exit.
func (c *Cascade) Dying() <-chan interface{} {
	return c.dying
}

// Dead provides a channel that will close once the Cascade is considered dead.
//
// This can be used as a signal to indicate when all goroutines have exited.
// However, actions may not have run yet
func (c *Cascade) Dead() <-chan interface{} {
	return c.dead
}

// Done provides a channel that will close once the Cascade is completely done.
//
// This can be used as a signal to indicate when all goroutines have exited and
// all actions have been completed.
func (c *Cascade) Done() <-chan interface{} {
	return c.done
}

// IsDead returns `true` if the Cascade has been cancelled or killed.
func (c *Cascade) IsDead() bool {
	c.muDead.RLock()
	defer c.muDead.RUnlock()
	return c.isDead
}

// Alive returns `true` if the Cascade has not been cancelled or killed.
func (c *Cascade) Alive() bool {
	return !c.IsDead()
}

// Kill will kill the Cascade and any children (just like the `Cancel` function) and will run
// any set actions.
//
// Note: This function blocks until all children and the specified Cascade have finished exiting.
func (c *Cascade) Kill() {
	c.muDead.Lock()
	if !c.isDead {
		c.isDead = true
		c.muDead.Unlock()
		wg := sync.WaitGroup{}
		c.muChildren.Lock()
		for child := range c.children {
			wg.Add(1)
			go func(ch *Cascade) {
				ch.Kill()
				wg.Done()
			}(child)
		}
		c.muChildren.Unlock()
		wg.Wait()
		c.closeAndClean(true)
	} else {
		c.muDead.Unlock()
	}
}

// KillWithError will kill the Cascade and any children (just like the `CancelWithError` function) and
// will run any set actions. The provided error will be set ONLY on the current Cascade.
//
// Notes:
//
// This function blocks until all children and the current Cascade have finished exiting.
//
// An error will be returned if an error has already been set on the current Cascade.
func (c *Cascade) KillWithError(err error) error {
	c.muErr.Lock()

	if c.err != nil {
		c.muErr.Unlock()
		return errors.New("cascade: error already set")
	}
	c.err = err
	c.muErr.Unlock()
	c.Kill()
	return nil
}

// Cancel will kill the Cascade and any children (just like the `Kill` function) but will not run
// any set actions.
//
// Note: This function blocks until all children and the specified Cascade have finished exiting.
func (c *Cascade) Cancel() {
	c.muDead.Lock()
	if !c.isDead {
		c.isDead = true
		c.muDead.Unlock()
		wg := sync.WaitGroup{}
		c.muChildren.Lock()
		for child := range c.children {
			wg.Add(1)
			go func(ch *Cascade) {
				ch.Cancel()
				wg.Done()
			}(child)
		}
		c.muChildren.Unlock()
		wg.Wait()
		c.closeAndClean(false)
	} else {
		c.muDead.Unlock()
	}
}

// CancelWithError will kill the Cascade and any children (just like the `KillWithError` function) but
// will not run any set actions. The provided error will be set ONLY on the current Cascade.
//
// Notes:
//
// This function blocks until all children and the current Cascade have finished exiting.
//
// An error will be returned if an error has already been set on the current Cascade.
func (c *Cascade) CancelWithError(err error) error {
	c.muErr.Lock()
	if c.err != nil {
		c.muErr.Unlock()
		return errors.New("cascade: error already set")
	}
	c.err = err
	c.muErr.Unlock()
	c.Cancel()
	return nil
}

// KillAll will `Kill` all Cascades in the whole tree from the `RootCascade` all the way to every
// child. All actions will be run.
//
// Note: This function blocks until ALL Cascades have been killed and finished exiting.
func (c *Cascade) KillAll() {
	if c.parent != nil {
		c.parent.KillAll()
	} else {
		// We found the root!
		c.Kill()
	}
}

// KillAllWithError will `Kill` all Cascades in the whole tree from the `RootCascade` all the way to every
// child. All actions will be run. The provided `error` is set ONLY on the `RootCascade`
//
// Notes:
//
// This function blocks until ALL Cascades have been killed and finished exiting.
//
// An error will be returned if an error has already been set on the current Cascade.
func (c *Cascade) KillAllWithError(err error) {
	if c.parent != nil {
		c.parent.KillAllWithError(err)
	} else {
		// We found the root!
		c.KillWithError(err)
	}
}

// CancelAll will `Cancel` all Cascades in the whole tree from the `RootCascade` all the way to every
// child. No actions will be run.
//
// Note: This function blocks until ALL Cascades have been cancelled and finished exiting.
func (c *Cascade) CancelAll() {
	if c.parent != nil {
		c.parent.CancelAll()
	} else {
		// We found the root!
		c.Cancel()
	}
}

// CancelAllWithError will `Cancel` all Cascades in the whole tree from the `RootCascade` all the way to every
// child. No actions will be run. The provided `error` is set ONLY on the `RootCascade`
//
// Notes:
//
// This function blocks until ALL Cascades have been cancelled and finished exiting.
//
// An error will be returned if an error has already been set on the current Cascade.
func (c *Cascade) CancelAllWithError(err error) {
	if c.parent != nil {
		c.parent.CancelAllWithError(err)
	} else {
		// We found the root!
		c.CancelWithError(err)
	}
}

// DoOnKill adds a function to the list of actions that should be performed when the Cascade is killed.
//
// Functions are added in a FIFO order and will be executed in order.
//
// Note: These actions will NOT be run if the Cascade is cancelled instead of killed.
func (c *Cascade) DoOnKill(action func()) {
	c.muActions.Lock()
	c.actions = append(c.actions, action)
	c.muActions.Unlock()
}

// DoFirstOnKill adds a function to the list of actions that should be performed when the Cascade is killed.
//
// Functions are added in a LIFO order and will be executed in order.
//
// Note: These actions will NOT be run if the Cascade is cancelled instead of killed.
func (c *Cascade) DoFirstOnKill(action func()) {
	c.muActions.Lock()
	c.actions = append([]func(){action}, c.actions...)
	c.muActions.Unlock()
}

// ChildCascade creates a new Cascade which is a child of the current Cascade.
//
// The child Cascade being killed or cancelled will not kill or cancel the parent.
func (c *Cascade) ChildCascade() *Cascade {
	child := RootCascade()
	child.parent = c
	c.muChildren.Lock()
	c.children[child] = nil
	c.muChildren.Unlock()
	return child
}

// Mark marks a goroutine as being tracked by a Cascade. It should be used similar to `Add` in `sync.WaitGroup`
// and called at the beginning of a goroutine.
//
// A marked goroutine MUST have an exit condition set for when the Cascade is in a dying state. Either `<-Dying()`
// or `Hold()` should be used.
//
// A marked goroutine MUST also `UnMark` once it has exited. This is most easily accomplished by using a `defer`.
//
// Example of a marked goroutine:
//  c := RootCascade()
//  go func() {
//  	c.Mark()          // Mark the goroutine as tracked
//  	defer c.UnMark()  // UnMark the goroutine once it's done.
//  	// Do something
//  	Hold()            // Wait for Cascade kill or cancel.
//  }()
//  // Additional Code Not Shown
func (c *Cascade) Mark() {
	c.muTracked.Lock()
	c.tracked++
	c.muTracked.Unlock()
	if c.IsDead() {
		c.muTracked.RLock()
		if c.tracked == 0 {
			c.onceDead.Do(func() {
				close(c.dead)
			})
		}
		c.muTracked.RUnlock()
	}
}

// UnMark removes the mark from a goroutine being tracked by a Cascade.
// It should be used similar to `Done` in `sync.WaitGroup`
// and called whenever a goroutine that has called `Mark` exits.
//
// See the docs for `Mark` for a usage example.
func (c *Cascade) UnMark() {
	c.muTracked.Lock()
	c.tracked--
	c.muTracked.Unlock()
	if c.IsDead() {
		c.muTracked.RLock()
		if c.tracked == 0 {
			c.onceDead.Do(func() {
				close(c.dead)
			})
		}
		c.muTracked.RUnlock()
	}
}

// Error returns the error set by one of the `WithError` functions.
func (c *Cascade) Error() error {
	c.muErr.Lock()
	defer c.muErr.Unlock()
	return c.err
}
