package cascade

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestRootCascade(t *testing.T) {
	cas := RootCascade()
	verifyCascadeEndState(t, cas, false, 0, false, 0, false, 0, false)
}

func TestCascade_Wrap(t *testing.T) {
	cas := RootCascade()
	mu := sync.Mutex{}
	wg := sync.WaitGroup{}

	didHold := true

	wg.Add(1)
	go cas.Wrap(func(c *Cascade) {
		wg.Done() // Signal that the function has started executing.
		c.Hold()
		mu.Lock()
		didHold = false
		mu.Unlock()
	})
	wg.Wait()                     // Make sure that the function has started executing.
	<-time.After(time.Second / 2) // Keep it held for a second to make sure we don't jump the gun.
	mu.Lock()
	if !didHold {
		t.Error("Wrap: Cascade did not hold until Kill!")
	}
	mu.Unlock()

	go cas.Kill()
	ok := didExitBeforeTime(cas, 1*time.Second)
	if !ok {
		t.Error("Wrap: Got stuck!")
	}
	verifyCascadeEndState(t, cas, false, 0, true, 0, false, 0, false)
}

func TestCascade_WrapInLoop(t *testing.T) {
	cas := RootCascade()
	wg := sync.WaitGroup{}
	once := sync.Once{}

	todo := func() {
		// This function checks something then returns.
		<-time.After(time.Second / 2)
		once.Do(func() {
			wg.Done() // Lets us wait for the function loop to run at least one time before we try to kill it.
		})
	}

	wg.Add(1)
	go cas.WrapInLoop(todo)

	wg.Wait() // Wait for the function loop to run at least once before killing
	go cas.Kill()
	ok := didExitBeforeTime(cas, 1*time.Second)
	if !ok {
		t.Error("WrapInLoop: Got stuck!")
	}
	verifyCascadeEndState(t, cas, false, 0, true, 0, false, 0, false)
}

func TestCascade_WrapInLoopWithBool(t *testing.T) {
	cas := RootCascade()
	wg := sync.WaitGroup{}
	onceTodo := sync.Once{}
	onceTodoExit := sync.Once{}

	todoCounter := 0
	todoExitCounter := 0
	muCounter := sync.Mutex{}
	muExitCounter := sync.Mutex{}

	todo := func() bool {
		// This function checks something then returns.
		<-time.After(time.Second / 2)
		muCounter.Lock()
		todoCounter++
		if todoCounter > 1 {
			onceTodo.Do(func() {
				wg.Done() // Lets us wait for the function loop to run at least one time before we try to kill it.
			})
		}
		muCounter.Unlock()
		return true // Do not break!
	}

	todoExit := func() bool {
		// This function checks something then returns.
		<-time.After(time.Second / 2)
		muExitCounter.Lock()
		todoExitCounter++
		muExitCounter.Unlock()
		onceTodoExit.Do(func() {
			wg.Done() // Lets us wait for the function loop to run at least one time before we try to kill it.
		})
		return false // This should break!
	}

	wg.Add(1)
	go cas.WrapInLoopWithBool(todo)
	wg.Add(1)
	go cas.WrapInLoopWithBool(todoExit)

	wg.Wait() // Wait for the function loop to run at least once before killing
	go cas.Kill()
	ok := didExitBeforeTime(cas, 2*time.Second)
	if !ok {
		t.Error("WrapInLoopWithBool: Got stuck!")
	}

	muCounter.Lock()
	if todoCounter < 2 {
		t.Error("WrapInLoopWithBool: Exited too early!")
	}
	muCounter.Unlock()
	muExitCounter.Lock()
	if todoExitCounter > 1 {
		t.Error("WrapInLoopWithBool: Ran again after exiting!")
	}
	muExitCounter.Unlock()

	verifyCascadeEndState(t, cas, false, 0, true, 0, false, 0, false)
}

func TestCascade_Go(t *testing.T) {
	cas := RootCascade()
	mu := sync.Mutex{}
	wg := sync.WaitGroup{}

	didHold := true

	wg.Add(1)
	cas.Go(func(c *Cascade) {
		wg.Done() // Signal that the function has started executing.
		c.Hold()
		mu.Lock()
		didHold = false
		mu.Unlock()
	})
	wg.Wait()                     // Make sure that the function has started executing.
	<-time.After(time.Second / 2) // Keep it held for a second to make sure we don't jump the gun.
	mu.Lock()
	if !didHold {
		t.Error("Go: Cascade did not hold until Kill!")
	}
	mu.Unlock()

	go cas.Kill()
	ok := didExitBeforeTime(cas, 1*time.Second)
	if !ok {
		t.Error("Go: Got stuck!")
	}

	verifyCascadeEndState(t, cas, false, 0, true, 0, false, 0, false)
}

func TestCascade_GoInLoop(t *testing.T) {
	cas := RootCascade()
	wg := sync.WaitGroup{}
	once := sync.Once{}
	todoCounter := 0
	muCounter := sync.Mutex{}

	todo := func() {
		// This function checks something then returns.
		<-time.After(time.Second / 2)
		muCounter.Lock()
		todoCounter++
		if todoCounter > 1 {
			once.Do(func() {
				wg.Done() // Lets us wait for the function loop to run at least one time before we try to kill it.
			})
		}
		muCounter.Unlock()
	}

	wg.Add(1)
	cas.GoInLoop(todo)

	wg.Wait() // Wait for the function loop to run at least once before killing
	go cas.Kill()
	ok := didExitBeforeTime(cas, 1*time.Second)
	if !ok {
		t.Error("GoInLoop: Got stuck!")
	}

	muCounter.Lock()
	if todoCounter < 2 {
		t.Error("GoInLoop: Exited too early!")
	}
	muCounter.Unlock()

	verifyCascadeEndState(t, cas, false, 0, true, 0, false, 0, false)
}

func TestCascade_GoInLoopWithBool(t *testing.T) {
	cas := RootCascade()
	wg := sync.WaitGroup{}
	onceTodo := sync.Once{}
	onceTodoExit := sync.Once{}

	todoCounter := 0
	todoExitCounter := 0
	muCounter := sync.Mutex{}
	muExitCounter := sync.Mutex{}

	todo := func() bool {
		// This function checks something then returns.
		<-time.After(time.Second / 2)
		muCounter.Lock()
		todoCounter++
		if todoCounter > 1 {
			onceTodo.Do(func() {
				wg.Done() // Lets us wait for the function loop to run at least one time before we try to kill it.
			})
		}
		muCounter.Unlock()
		return true // Do not break!
	}

	todoExit := func() bool {
		// This function checks something then returns.
		<-time.After(time.Second / 2)
		muExitCounter.Lock()
		todoExitCounter++
		muExitCounter.Unlock()
		onceTodoExit.Do(func() {
			wg.Done() // Lets us wait for the function loop to run at least one time before we try to kill it.
		})
		return false // This should break!
	}

	wg.Add(1)
	cas.GoInLoopWithBool(todo)
	wg.Add(1)
	cas.GoInLoopWithBool(todoExit)

	wg.Wait() // Wait for the function loop to run at least once before killing
	go cas.Kill()
	ok := didExitBeforeTime(cas, 2*time.Second)
	if !ok {
		t.Error("WrapInLoopWithBool: Got stuck!")
	}

	muCounter.Lock()
	if todoCounter < 2 {
		t.Error("WrapInLoopWithBool: Exited too early!")
	}
	muCounter.Unlock()
	muExitCounter.Lock()
	if todoExitCounter > 1 {
		t.Error("WrapInLoopWithBool: Ran again after exiting!")
	}
	muExitCounter.Unlock()

	verifyCascadeEndState(t, cas, false, 0, true, 0, false, 0, false)
}

func TestCascade_Hold(t *testing.T) {
	cas := RootCascade()
	wg := sync.WaitGroup{}
	done := false
	mu := sync.Mutex{}

	wg.Add(1)
	go func() {
		cas.Mark()
		defer cas.UnMark()
		wg.Done() // Just making sure that the goroutine gets to the hold.
		cas.Hold()
		mu.Lock()
		done = true
		mu.Unlock()
	}()

	wg.Wait()
	<-time.After(time.Second / 2) // Give some time to make sure that the hold holds
	mu.Lock()
	if done {
		t.Error("Hold: Did not hold!")
	}
	mu.Unlock()
	go cas.Kill()
	ok := didExitBeforeTime(cas, time.Second/2)
	if !ok {
		t.Error("Hold: Wait timed out!")
	}
	mu.Lock()
	if !done {
		t.Error("Hold failed to release!")
	}
	mu.Unlock()

	verifyCascadeEndState(t, cas, false, 0, true, 0, false, 0, false)
}

func TestCascade_Wait(t *testing.T) {
	cas := RootCascade()

	go func() {
		cas.Mark()
		defer cas.UnMark()
		cas.Hold()
	}()

	go cas.Kill()
	ok := didExitBeforeTime(cas, time.Second/2)
	if !ok {
		t.Error("Wait: Wait timed out!")
	}
	verifyCascadeEndState(t, cas, false, 0, true, 0, false, 0, false)
}

func TestCascade_WaitDone(t *testing.T) {
	cas := RootCascade()
	waiter := make(chan struct{}, 0)

	action := func() {
		<-waiter
	}

	cas.DoOnKill(action)

	go func() {
		cas.Mark()
		defer cas.UnMark()
		cas.Hold()
	}()

	go cas.Kill()
	ok := didExitBeforeTime(cas, time.Second/2)
	if !ok {
		t.Error("WaitDone: Wait timed out!")
	}
	select {
	case <-cas.done:
		t.Error("WaitDone: Didn't wait for actions to exit!")
	default:
	}
	close(waiter)
	cas.WaitDone()
	select {
	case <-cas.done:
	default:
		t.Error("WaitDone: Not actually done after done!")
	}

	verifyCascadeEndState(t, cas, false, 0, true, 1, false, 0, false)
}

func TestCascade_DeadAndDone(t *testing.T) {
	cas := RootCascade()
	waiter := make(chan struct{}, 0)

	action := func() {
		<-waiter
	}

	cas.DoOnKill(action)

	go func() {
		cas.Mark()
		defer cas.UnMark()
		cas.Hold()
	}()

	go cas.Kill()
	ok := false
	select {
	case <-cas.Dead():
		ok = true
	case <-time.After(time.Second / 2):
	}
	if !ok {
		t.Error("DeadAndDone: Dead timed out!")
	}
	select {
	case <-cas.Done():
		t.Error("DeadAndDone: Done didn't wait for actions to exit!")
	default:
	}
	close(waiter)
	select {
	case <-cas.Done():
	case <-time.After(time.Second / 2):
		t.Error("DeadAndDone: Not actually done after done!")
	}

	verifyCascadeEndState(t, cas, false, 0, true, 1, false, 0, false)
}

func TestCascade_Dying(t *testing.T) {
	cas := RootCascade()

	go func() {
		cas.Mark()
		defer cas.UnMark()
		<-cas.Dying()
	}()

	go cas.Kill()
	ok := didExitBeforeTime(cas, time.Second/2)
	if !ok {
		t.Error("Dying: Wait timed out!")
	}
	verifyCascadeEndState(t, cas, false, 0, true, 0, false, 0, false)
}

func TestCascade_IsDead(t *testing.T) {
	cas := RootCascade()

	if cas.IsDead() {
		t.Error("IsDead: IsDead before Killed!")
	}

	go cas.Kill()
	ok := didExitBeforeTime(cas, time.Second/2)
	if !ok {
		t.Error("IsDead: Wait timed out!")
	}

	if !cas.IsDead() {
		t.Error("IsDead: Not IsDead after Killed!")
	}

	verifyCascadeEndState(t, cas, false, 0, true, 0, false, 0, false)
}

func TestCascade_Alive(t *testing.T) {
	cas := RootCascade()

	if !cas.Alive() {
		t.Error("Alive: Not Alive before Killed!")
	}

	go cas.Kill()
	ok := didExitBeforeTime(cas, time.Second/2)
	if !ok {
		t.Error("Alive: Wait timed out!")
	}

	if cas.Alive() {
		t.Error("Alive: Alive after Killed!")
	}

	verifyCascadeEndState(t, cas, false, 0, true, 0, false, 0, false)
}

func TestCascade_Kill(t *testing.T) {
	cas := RootCascade()
	child := cas.ChildCascade()
	mu := sync.Mutex{}
	count := 2
	didAction1 := false
	didAction2 := false
	didAction3 := false
	muAction1 := sync.Mutex{}
	muAction2 := sync.Mutex{}
	muAction3 := sync.Mutex{}
	waitForKill := make(chan struct{}, 0)

	action1 := func() {
		muAction1.Lock()
		didAction1 = true
		muAction1.Unlock()
	}

	action2 := func() {
		muAction2.Lock()
		didAction2 = true
		muAction2.Unlock()
		muAction1.Lock()
		if !didAction1 {
			t.Error("Kill: Action2 ran before Action1")
		}
		muAction1.Unlock()
	}

	action3 := func() {
		muAction3.Lock()
		didAction3 = true
		muAction3.Unlock()
		muAction1.Lock()
		if !didAction1 {
			t.Error("Kill: Action3 ran before Action1!")
		}
		muAction1.Unlock()
		muAction2.Lock()
		if !didAction2 {
			t.Error("Kill: Action3 ran before Action2!")
		}
		muAction2.Unlock()
	}

	cas.DoOnKill(action3)
	cas.DoFirstOnKill(action2)
	child.DoOnKill(action1)

	go func() {
		cas.Mark()
		defer func() {
			mu.Lock()
			count--
			mu.Unlock()
			cas.UnMark()
		}()

		<-cas.Dying()
	}()

	go func() {
		child.Mark()
		defer func() {
			mu.Lock()
			count--
			mu.Unlock()
			child.UnMark()
		}()

		<-child.Dying()
	}()

	go func() {
		cas.Kill()
		close(waitForKill)
	}()
	select {
	case <-waitForKill:
	case <-time.After(1 * time.Second):
		t.Error("Kill: Got stuck in Kill Command!")
	}
	ok := didExitBeforeTime(cas, 1*time.Second)
	if !ok {
		t.Error("Kill: Got stuck in Kill!")
	}
	mu.Lock()
	if count != 0 {
		t.Error("Kill: Not all goroutines were killed!")
	}
	mu.Unlock()
	muAction1.Lock()
	if !didAction1 {
		t.Error("Kill: Action1 was not executed!")
	}
	muAction1.Unlock()
	muAction2.Lock()
	if !didAction2 {
		t.Error("Kill: Action2 was not executed!")
	}
	muAction2.Unlock()
	muAction3.Lock()
	if !didAction3 {
		t.Error("Kill: Action3 was not executed!")
	}
	muAction3.Unlock()

	verifyCascadeEndState(t, cas, false, 0, true, 2, false, 0, false)
	verifyCascadeEndState(t, child, true, 0, true, 1, false, 0, false)
}

func TestCascade_KillWithNoTracked(t *testing.T) {
	cas := RootCascade()
	go cas.Kill() // Poor thing didn't even have a chance to track anything :(
	ok := didExitBeforeTime(cas, 1*time.Second)
	if !ok {
		t.Error("KillWithNoTracked: Got stuck!")
	}
	verifyCascadeEndState(t, cas, false, 0, true, 0, false, 0, false)
}

func TestCascade_KillAfterKill(t *testing.T) {
	cas := RootCascade()
	go cas.Kill() // Poor thing didn't even have a chance to track anything :(
	ok := didExitBeforeTime(cas, 1*time.Second)
	if !ok {
		t.Error("KillAfterKill: Got stuck!")
	}
	verifyCascadeEndState(t, cas, false, 0, true, 0, false, 0, false)
	cas.Kill() // Should do nothing!
}

func TestCascade_KillWithError(t *testing.T) {
	cas := RootCascade()
	err := errors.New("kill")
	casErr := cas.KillWithError(err)
	if casErr != nil {
		t.Error("KillWithError: Unable to set error!")
	}
	ok := didExitBeforeTime(cas, time.Second/2)
	if !ok {
		t.Error("KillWithError: Got stuck!")
	}
	if err != cas.Error() {
		t.Error("KillWithError: Error does not match!")
	}
	verifyCascadeEndState(t, cas, false, 0, true, 0, false, 0, true)
}

func TestCascade_KillWithErrorWithError(t *testing.T) {
	cas := RootCascade()
	err := errors.New("kill")
	cas.muErr.Lock()
	cas.err = errors.New("another error")
	cas.muErr.Unlock()
	casErr := cas.KillWithError(err)
	if casErr == nil {
		t.Error("KillWithErrorWithError: Didn't get error for incorrect Kill!")
	}
	go cas.Kill()
	ok := didExitBeforeTime(cas, time.Second/2)
	if !ok {
		t.Error("KillWithErrorWithError: Got stuck!")
	}
	verifyCascadeEndState(t, cas, false, 0, true, 0, false, 0, true)
}

func TestCascade_Cancel(t *testing.T) {
	cas := RootCascade()
	child := cas.ChildCascade()
	mu := sync.Mutex{}
	count := 2
	didAction1 := false
	didAction2 := false
	didAction3 := false
	muAction1 := sync.Mutex{}
	muAction2 := sync.Mutex{}
	muAction3 := sync.Mutex{}
	waitForCancel := make(chan struct{}, 0)

	action1 := func() {
		muAction1.Lock()
		didAction1 = true
		muAction1.Unlock()
		t.Error("Cancel: Action1 ran!")
	}

	action2 := func() {
		muAction2.Lock()
		didAction2 = true
		muAction2.Unlock()
		muAction1.Lock()
		if !didAction1 {
			t.Error("Cancel: Action2 ran before Action1")
		}
		muAction1.Unlock()
		t.Error("Cancel: Action2 ran!")
	}

	action3 := func() {
		muAction3.Lock()
		didAction3 = true
		muAction3.Unlock()
		muAction1.Lock()
		if !didAction1 {
			t.Error("Cancel: Action3 ran before Action1!")
		}
		muAction1.Unlock()
		muAction2.Lock()
		if !didAction2 {
			t.Error("Cancel: Action3 ran before Action2!")
		}
		muAction2.Unlock()
		t.Error("Cancel: Action3 ran!")
	}

	cas.DoOnKill(action3)
	cas.DoFirstOnKill(action2)
	child.DoOnKill(action1)

	go func() {
		cas.Mark()
		defer func() {
			mu.Lock()
			count--
			mu.Unlock()
			cas.UnMark()
		}()

		<-cas.Dying()
	}()

	go func() {
		child.Mark()
		defer func() {
			mu.Lock()
			count--
			mu.Unlock()
			child.UnMark()
		}()

		<-child.Dying()
	}()

	go func() {
		cas.Cancel()
		close(waitForCancel)
	}()
	select {
	case <-waitForCancel:
	case <-time.After(1 * time.Second):
		t.Error("Cancel: Got stuck in Cancel Command!")
	}
	ok := didExitBeforeTime(cas, 1*time.Second)
	if !ok {
		t.Error("Cancel: Got stuck in Cancel!")
	}
	mu.Lock()
	if count != 0 {
		t.Error("Cancel: Not all goroutines were killed!")
	}
	mu.Unlock()
	muAction1.Lock()
	if didAction1 {
		t.Error("Cancel: Action1 was executed!")
	}
	muAction1.Unlock()
	muAction2.Lock()
	if didAction2 {
		t.Error("Cancel: Action2 was executed!")
	}
	muAction2.Unlock()
	muAction3.Lock()
	if didAction3 {
		t.Error("Cancel: Action3 was executed!")
	}
	muAction3.Unlock()

	verifyCascadeEndState(t, cas, false, 0, true, 2, false, 0, false)
	verifyCascadeEndState(t, child, true, 0, true, 1, false, 0, false)
}

func TestCascade_CancelWithNoTracked(t *testing.T) {
	cas := RootCascade()
	go cas.Cancel() // Poor thing didn't even have a chance to track anything :(
	ok := didExitBeforeTime(cas, 1*time.Second)
	if !ok {
		t.Error("CancelWithNoTracked: Got stuck!")
	}
	verifyCascadeEndState(t, cas, false, 0, true, 0, false, 0, false)
}

func TestCascade_CancelAfterCancel(t *testing.T) {
	cas := RootCascade()
	go cas.Cancel() // Poor thing didn't even have a chance to track anything :(
	ok := didExitBeforeTime(cas, 1*time.Second)
	if !ok {
		t.Error("CancelAfterCancel: Got stuck!")
	}
	verifyCascadeEndState(t, cas, false, 0, true, 0, false, 0, false)
	cas.Cancel() // Should do nothing!
}

func TestCascade_CancelWithError(t *testing.T) {
	cas := RootCascade()
	err := errors.New("cancel")
	go cas.CancelWithError(err)
	ok := didExitBeforeTime(cas, time.Second/2)
	if !ok {
		t.Error("CancelWithError: Got stuck!")
	}
	if err != cas.Error() {
		t.Error("CancelWithError: Error does not match!")
	}
	verifyCascadeEndState(t, cas, false, 0, true, 0, false, 0, true)
}

func TestCascade_CancelWithErrorWithError(t *testing.T) {
	cas := RootCascade()
	err := errors.New("cancel")
	cas.muErr.Lock()
	cas.err = errors.New("another error")
	cas.muErr.Unlock()
	casErr := cas.CancelWithError(err)
	if casErr == nil {
		t.Error("CancelWithErrorWithError: Didn't get error for incorrect Kill!")
	}
	go cas.Cancel()
	ok := didExitBeforeTime(cas, time.Second/2)
	if !ok {
		t.Error("CancelWithErrorWithError: Got stuck!")
	}
	verifyCascadeEndState(t, cas, false, 0, true, 0, false, 0, true)
}

func TestCascade_KillAll(t *testing.T) {
	cas := RootCascade()
	child1 := cas.ChildCascade()
	child2 := child1.ChildCascade()
	child3 := child1.ChildCascade()
	didAction := false
	muAction := sync.Mutex{}
	waitForKill := make(chan struct{}, 0)

	action := func() {
		muAction.Lock()
		didAction = true
		muAction.Unlock()
	}

	cas.DoOnKill(action)

	verifyCascadeEndState(t, cas, false, 1, false, 1, false, 0, false)
	verifyCascadeEndState(t, child1, true, 2, false, 0, false, 0, false)
	verifyCascadeEndState(t, child2, true, 0, false, 0, false, 0, false)
	verifyCascadeEndState(t, child3, true, 0, false, 0, false, 0, false)

	go func() {
		child3.KillAll()
		close(waitForKill)
	}()
	select {
	case <-waitForKill:
	case <-time.After(1 * time.Second):
		t.Error("KillAll: Got stuck in Kill Command!")
	}

	okCas := didExitBeforeTime(cas, time.Second/2)
	if !okCas {
		t.Error("KillAll: Cas got stuck!")
	}
	okChild1 := didExitBeforeTime(child1, time.Second/2)
	if !okChild1 {
		t.Error("KillAll: Child1 got stuck!")
	}
	okChild2 := didExitBeforeTime(child2, time.Second/2)
	if !okChild2 {
		t.Error("KillAll: Child2 got stuck!")
	}
	okChild3 := didExitBeforeTime(child3, time.Second/2)
	if !okChild3 {
		t.Error("KillAll: Child3 got stuck!")
	}

	muAction.Lock()
	if !didAction {
		t.Error("KillAllWithError: Action was not run!")
	}
	muAction.Unlock()

	verifyCascadeEndState(t, cas, false, 0, true, 1, false, 0, false)
	verifyCascadeEndState(t, child1, true, 0, true, 0, false, 0, false)
	verifyCascadeEndState(t, child2, true, 0, true, 0, false, 0, false)
	verifyCascadeEndState(t, child3, true, 0, true, 0, false, 0, false)
}

func TestCascade_KillAllWithError(t *testing.T) {
	cas := RootCascade()
	child1 := cas.ChildCascade()
	child2 := child1.ChildCascade()
	child3 := child1.ChildCascade()
	err := errors.New("kill all with error")
	didAction := false
	muAction := sync.Mutex{}
	waitForKill := make(chan struct{}, 0)

	action := func() {
		muAction.Lock()
		didAction = true
		muAction.Unlock()
	}

	cas.DoOnKill(action)

	verifyCascadeEndState(t, cas, false, 1, false, 1, false, 0, false)
	verifyCascadeEndState(t, child1, true, 2, false, 0, false, 0, false)
	verifyCascadeEndState(t, child2, true, 0, false, 0, false, 0, false)
	verifyCascadeEndState(t, child3, true, 0, false, 0, false, 0, false)

	go func() {
		child3.KillAllWithError(err)
		close(waitForKill)
	}()
	select {
	case <-waitForKill:
	case <-time.After(1 * time.Second):
		t.Error("KillAllWithError: Got stuck in Kill Command!")
	}

	okCas := didExitBeforeTime(cas, time.Second/2)
	if !okCas {
		t.Error("KillAllWithError: Cas got stuck!")
	}
	okChild1 := didExitBeforeTime(child1, time.Second/2)
	if !okChild1 {
		t.Error("KillAllWithError: Child1 got stuck!")
	}
	okChild2 := didExitBeforeTime(child2, time.Second/2)
	if !okChild2 {
		t.Error("KillAllWithError: Child2 got stuck!")
	}
	okChild3 := didExitBeforeTime(child3, time.Second/2)
	if !okChild3 {
		t.Error("KillAllWithError: Child3 got stuck!")
	}

	if err != cas.Error() {
		t.Error("KillAllWithError: Error on root does not match!")
	}

	muAction.Lock()
	if !didAction {
		t.Error("KillAllWithError: Action was not run!")
	}
	muAction.Unlock()

	verifyCascadeEndState(t, cas, false, 0, true, 1, false, 0, true)
	verifyCascadeEndState(t, child1, true, 0, true, 0, false, 0, false)
	verifyCascadeEndState(t, child2, true, 0, true, 0, false, 0, false)
	verifyCascadeEndState(t, child3, true, 0, true, 0, false, 0, false)
}

func TestCascade_CancelAll(t *testing.T) {
	cas := RootCascade()
	child1 := cas.ChildCascade()
	child2 := child1.ChildCascade()
	child3 := child1.ChildCascade()
	didAction := false
	muAction := sync.Mutex{}
	waitForCancel := make(chan struct{}, 0)

	action := func() {
		muAction.Lock()
		didAction = true
		muAction.Unlock()
		t.Error("CancelAll: Action was run!")
	}

	cas.DoOnKill(action)

	verifyCascadeEndState(t, cas, false, 1, false, 1, false, 0, false)
	verifyCascadeEndState(t, child1, true, 2, false, 0, false, 0, false)
	verifyCascadeEndState(t, child2, true, 0, false, 0, false, 0, false)
	verifyCascadeEndState(t, child3, true, 0, false, 0, false, 0, false)

	go func() {
		child3.CancelAll()
		close(waitForCancel)
	}()
	select {
	case <-waitForCancel:
	case <-time.After(1 * time.Second):
		t.Error("CancelAll: Got stuck in Kill Command!")
	}

	okCas := didExitBeforeTime(cas, time.Second/2)
	if !okCas {
		t.Error("CancelAll: Cas got stuck!")
	}
	okChild1 := didExitBeforeTime(child1, time.Second/2)
	if !okChild1 {
		t.Error("CancelAll: Child1 got stuck!")
	}
	okChild2 := didExitBeforeTime(child2, time.Second/2)
	if !okChild2 {
		t.Error("CancelAll: Child2 got stuck!")
	}
	okChild3 := didExitBeforeTime(child3, time.Second/2)
	if !okChild3 {
		t.Error("CancelAll: Child3 got stuck!")
	}

	muAction.Lock()
	if didAction {
		t.Error("CancelAll: Action was run!")
	}
	muAction.Unlock()

	verifyCascadeEndState(t, cas, false, 0, true, 1, false, 0, false)
	verifyCascadeEndState(t, child1, true, 0, true, 0, false, 0, false)
	verifyCascadeEndState(t, child2, true, 0, true, 0, false, 0, false)
	verifyCascadeEndState(t, child3, true, 0, true, 0, false, 0, false)
}

func TestCascade_CancelAllWithError(t *testing.T) {
	cas := RootCascade()
	child1 := cas.ChildCascade()
	child2 := child1.ChildCascade()
	child3 := child1.ChildCascade()
	err := errors.New("cancel all with error")
	didAction := false
	muAction := sync.Mutex{}
	waitForCancel := make(chan struct{}, 0)

	action := func() {
		muAction.Lock()
		didAction = true
		muAction.Unlock()
		t.Error("CancelAllWithError: Action was run!")
	}

	cas.DoOnKill(action)

	verifyCascadeEndState(t, cas, false, 1, false, 1, false, 0, false)
	verifyCascadeEndState(t, child1, true, 2, false, 0, false, 0, false)
	verifyCascadeEndState(t, child2, true, 0, false, 0, false, 0, false)
	verifyCascadeEndState(t, child3, true, 0, false, 0, false, 0, false)

	go func() {
		child3.CancelAllWithError(err)
		close(waitForCancel)
	}()
	select {
	case <-waitForCancel:
	case <-time.After(1 * time.Second):
		t.Error("CancelAllWithError: Got stuck in Kill Command!")
	}

	okCas := didExitBeforeTime(cas, time.Second/2)
	if !okCas {
		t.Error("CancelAllWithError: Cas got stuck!")
	}
	okChild1 := didExitBeforeTime(child1, time.Second/2)
	if !okChild1 {
		t.Error("CancelAllWithError: Child1 got stuck!")
	}
	okChild2 := didExitBeforeTime(child2, time.Second/2)
	if !okChild2 {
		t.Error("CancelAllWithError: Child2 got stuck!")
	}
	okChild3 := didExitBeforeTime(child3, time.Second/2)
	if !okChild3 {
		t.Error("CancelAllWithError: Child3 got stuck!")
	}

	if err != cas.Error() {
		t.Error("CancelAllWithError: Error on root does not match!")
	}

	muAction.Lock()
	if didAction {
		t.Error("CancelAllWithError: Action was run!")
	}
	muAction.Unlock()

	verifyCascadeEndState(t, cas, false, 0, true, 1, false, 0, true)
	verifyCascadeEndState(t, child1, true, 0, true, 0, false, 0, false)
	verifyCascadeEndState(t, child2, true, 0, true, 0, false, 0, false)
	verifyCascadeEndState(t, child3, true, 0, true, 0, false, 0, false)
}

func TestCascade_DoOnKill(t *testing.T) {
	cas := RootCascade()
	didAction1 := false
	didAction2 := false
	didAction3 := false
	muAction1 := sync.Mutex{}
	muAction2 := sync.Mutex{}
	muAction3 := sync.Mutex{}
	waitForKill := make(chan struct{}, 0)

	action1 := func() {
		muAction1.Lock()
		didAction1 = true
		muAction1.Unlock()
	}

	action2 := func() {
		muAction2.Lock()
		didAction2 = true
		muAction2.Unlock()
		muAction1.Lock()
		if !didAction1 {
			t.Error("Kill: Action2 ran before Action1")
		}
		muAction1.Unlock()
	}

	action3 := func() {
		muAction3.Lock()
		didAction3 = true
		muAction3.Unlock()
		muAction1.Lock()
		if !didAction1 {
			t.Error("Kill: Action3 ran before Action1!")
		}
		muAction1.Unlock()
		muAction2.Lock()
		if !didAction2 {
			t.Error("Kill: Action3 ran before Action2!")
		}
		muAction2.Unlock()
	}

	cas.DoOnKill(action1)
	cas.DoOnKill(action2)
	cas.DoOnKill(action3)

	go func() {
		cas.Kill()
		close(waitForKill)
	}()
	select {
	case <-waitForKill:
	case <-time.After(1 * time.Second):
		t.Error("DoOnKill: Got stuck in Kill Command!")
	}
	ok := didExitBeforeTime(cas, 1*time.Second)
	if !ok {
		t.Error("DoOnKill: Got stuck in Kill!")
	}
	muAction1.Lock()
	if !didAction1 {
		t.Error("DoOnKill: Action1 was not executed!")
	}
	muAction1.Unlock()
	muAction2.Lock()
	if !didAction2 {
		t.Error("DoOnKill: Action2 was not executed!")
	}
	muAction2.Unlock()
	muAction3.Lock()
	if !didAction3 {
		t.Error("DoOnKill: Action3 was not executed!")
	}
	muAction3.Unlock()

	verifyCascadeEndState(t, cas, false, 0, true, 3, false, 0, false)
}

func TestCascade_DoFirstOnKill(t *testing.T) {
	cas := RootCascade()
	didAction1 := false
	didAction2 := false
	didAction3 := false
	muAction1 := sync.Mutex{}
	muAction2 := sync.Mutex{}
	muAction3 := sync.Mutex{}
	waitForKill := make(chan struct{}, 0)

	action1 := func() {
		muAction1.Lock()
		didAction1 = true
		muAction1.Unlock()
	}

	action2 := func() {
		muAction2.Lock()
		didAction2 = true
		muAction2.Unlock()
		muAction1.Lock()
		if !didAction1 {
			t.Error("Kill: Action2 ran before Action1")
		}
		muAction1.Unlock()
	}

	action3 := func() {
		muAction3.Lock()
		didAction3 = true
		muAction3.Unlock()
		muAction1.Lock()
		if !didAction1 {
			t.Error("Kill: Action3 ran before Action1!")
		}
		muAction1.Unlock()
		muAction2.Lock()
		if !didAction2 {
			t.Error("Kill: Action3 ran before Action2!")
		}
		muAction2.Unlock()
	}

	cas.DoFirstOnKill(action3)
	cas.DoFirstOnKill(action2)
	cas.DoFirstOnKill(action1)

	go func() {
		cas.Kill()
		close(waitForKill)
	}()
	select {
	case <-waitForKill:
	case <-time.After(1 * time.Second):
		t.Error("DoFirstOnKill: Got stuck in Kill Command!")
	}
	ok := didExitBeforeTime(cas, 1*time.Second)
	if !ok {
		t.Error("DoFirstOnKill: Got stuck in Kill!")
	}
	muAction1.Lock()
	if !didAction1 {
		t.Error("DoFirstOnKill: Action1 was not executed!")
	}
	muAction1.Unlock()
	muAction2.Lock()
	if !didAction2 {
		t.Error("DoFirstOnKill: Action2 was not executed!")
	}
	muAction2.Unlock()
	muAction3.Lock()
	if !didAction3 {
		t.Error("DoFirstOnKill: Action3 was not executed!")
	}
	muAction3.Unlock()

	verifyCascadeEndState(t, cas, false, 0, true, 3, false, 0, false)
}

func TestCascade_ChildCascade(t *testing.T) {
	cas := RootCascade()
	child1 := cas.ChildCascade()
	child2 := child1.ChildCascade()
	child3 := child1.ChildCascade()
	child4 := child3.ChildCascade()
	child5 := child3.ChildCascade()
	child6 := child3.ChildCascade()

	if child1.parent != cas {
		t.Error("ChildCascade: Parent Did Not Match!")
	}
	if child2.parent != child1 {
		t.Error("ChildCascade: Parent Did Not Match!")
	}
	if child3.parent != child1 {
		t.Error("ChildCascade: Parent Did Not Match!")
	}
	if child4.parent != child3 {
		t.Error("ChildCascade: Parent Did Not Match!")
	}
	if child5.parent != child3 {
		t.Error("ChildCascade: Parent Did Not Match!")
	}
	if child6.parent != child3 {
		t.Error("ChildCascade: Parent Did Not Match!")
	}

	cas.muChildren.Lock()
	if _, ok := cas.children[child1]; !ok {
		t.Error("ChildCascade: Child Did Not Match!")
	}
	cas.muChildren.Unlock()
	child1.muChildren.Lock()
	if _, ok := child1.children[child2]; !ok {
		t.Error("ChildCascade: Child Did Not Match!")
	}
	if _, ok := child1.children[child3]; !ok {
		t.Error("ChildCascade: Child Did Not Match!")
	}
	child1.muChildren.Unlock()
	child3.muChildren.Lock()
	if _, ok := child3.children[child4]; !ok {
		t.Error("ChildCascade: Child Did Not Match!")
	}
	if _, ok := child3.children[child5]; !ok {
		t.Error("ChildCascade: Child Did Not Match!")
	}
	if _, ok := child3.children[child6]; !ok {
		t.Error("ChildCascade: Child Did Not Match!")
	}
	child3.muChildren.Unlock()

	verifyCascadeEndState(t, cas, false, 1, false, 0, false, 0, false)
	verifyCascadeEndState(t, child1, true, 2, false, 0, false, 0, false)
	verifyCascadeEndState(t, child2, true, 0, false, 0, false, 0, false)
	verifyCascadeEndState(t, child3, true, 3, false, 0, false, 0, false)
	verifyCascadeEndState(t, child4, true, 0, false, 0, false, 0, false)
	verifyCascadeEndState(t, child5, true, 0, false, 0, false, 0, false)
	verifyCascadeEndState(t, child6, true, 0, false, 0, false, 0, false)
}

func TestCascade_Mark(t *testing.T) {
	cas := RootCascade()
	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		cas.Mark()
		defer cas.UnMark()
		wg.Done()
		cas.Hold()
	}()

	wg.Wait()
	cas.muTracked.Lock()
	if cas.tracked != 1 {
		t.Error("Mark: Mark did not increment Tracking!")
	}
	cas.muTracked.Unlock()

	go cas.Kill()
	ok := didExitBeforeTime(cas, 1*time.Second)
	if !ok {
		t.Error("Mark: Got stuck in Kill!")
	}

	verifyCascadeEndState(t, cas, false, 0, true, 0, false, 0, false)

	// Check that nothing happens on a dead Cascade...
	func() {
		cas.Mark()
		cas.UnMark()
	}()
}

func TestCascade_MarkEmptyDead(t *testing.T) {
	cas := RootCascade()

	cas.UnMark() // This will create a condition where Kill will not immediately be able to exit.

	go cas.Kill()
	select {
	case <-cas.Dying():
	case <-time.After(1 * time.Second):
		t.Error("MarkEmptyDead: Got stuck in Dying!")
	}

	cas.Mark() // This will simulate a late goroutine connecting...

	ok := didExitBeforeTime(cas, 1*time.Second)
	if !ok {
		t.Error("MarkEmptyDead: Got stuck in Kill!")
	}

	verifyCascadeEndState(t, cas, false, 0, true, 0, false, 0, false)

	// Check that nothing happens on a dead Cascade...
	func() {
		cas.Mark()
		cas.UnMark()
	}()
}

func TestCascade_UnMark(t *testing.T) {
	cas := RootCascade()
	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		cas.Mark()
		defer cas.UnMark()
		wg.Done()
		cas.Hold()
	}()

	wg.Wait()

	go cas.Kill()
	ok := didExitBeforeTime(cas, 1*time.Second)
	if !ok {
		t.Error("UnMark: Got stuck in Kill!")
	}

	cas.muTracked.Lock()
	if cas.tracked != 0 {
		t.Error("UnMark: UnMark did not decrement Tracking!")
	}
	cas.muTracked.Unlock()

	verifyCascadeEndState(t, cas, false, 0, true, 0, false, 0, false)
}

func TestCascade_Error(t *testing.T) {
	cas := RootCascade()
	err := errors.New("error")

	cas.muErr.Lock()
	currentErr := cas.err
	cas.muErr.Unlock()
	if currentErr != nil {
		t.Error("Error: Error is not nil!")
	}
	if currentErr != cas.Error() {
		t.Error("Error: Error does not match!")
	}

	go cas.KillWithError(err)

	ok := didExitBeforeTime(cas, 1*time.Second)
	if !ok {
		t.Error("Error: Got stuck in Kill!")
	}

	cas.muErr.Lock()
	currentErr = cas.err
	cas.muErr.Unlock()

	if currentErr != err {
		t.Error("Error: Error did not get set!")
	}
	if currentErr != cas.Error() {
		t.Error("Error: Error does not match!")
	}

	verifyCascadeEndState(t, cas, false, 0, true, 0, false, 0, true)
}

func didExitBeforeTime(c *Cascade, d time.Duration) bool {
	select {
	case <-c.dead:
		return true
	case <-time.After(d):
		return false
	}
}

// For int values, negative means it doesn't matter
func verifyCascadeEndState(t *testing.T, c *Cascade, hasParent bool, numChildren int, wantDead bool, numActions int, hasContext bool, numTrackedContexts int, hasError bool) {

	if hasParent == (c.parent == nil) {
		t.Errorf("Cascade Should Have Parent: %v, Cascade has Parent: %v", hasParent, c.parent != nil)
	}

	c.muChildren.Lock()
	if numChildren >= 0 {
		if len(c.children) != numChildren {
			t.Errorf("Cascade should have %v Children, it has %v", numChildren, len(c.children))
		}
	}
	c.muChildren.Unlock()

	isDead := false

	select {
	case <-c.Dying():
		if !wantDead {
			t.Error("c.Dying() should block but did not!")
		}
		isDead = true
	default:
		if wantDead {
			t.Error("c.Dying() did block but should not have!")
		}
	}

	if c.IsDead() != wantDead {
		t.Error("c.IsDead() does not appear to match want!")
	}

	if c.IsDead() != isDead {
		t.Error("c.Dying() and c.IsDead() did not match results!")
	}

	if wantDead && isDead && c.IsDead() {
		waitChan := make(chan struct{})
		go func() {
			defer close(waitChan)
			c.Wait()
		}()
		select {
		case <-waitChan:
		case <-time.After(2 * time.Second):
			t.Error("c.Wait() Timed Out!")
		}

		holdChan := make(chan struct{})
		go func() {
			defer close(holdChan)
			c.Hold()
		}()
		select {
		case <-holdChan:
		case <-time.After(2 * time.Second):
			t.Error("c.Hold() Timed Out!")
		}
	}

	if numActions >= 0 {
		if len(c.actions) != numActions {
			t.Errorf("Cascade should have %v Actions, it has %v", numActions, len(c.actions))
		}
	}

	if hasContext == (c.ctx == nil) {
		t.Errorf("Cascade Should Have Context: %v, Cascade has Context: %v", hasContext, c.ctx != nil)
	}

	c.muCtx.Lock()
	if numTrackedContexts >= 0 {
		if len(c.trackedCtx) != numTrackedContexts {
			t.Errorf("Cascade should have %v Tracked Contexts, it has %v", numTrackedContexts, len(c.trackedCtx))
		}
	}
	c.muCtx.Unlock()

	if hasError == (c.err == nil) {
		if c.err != nil {
			t.Errorf("Cascade Should Have Error: %v, Cascade has Error: %v", hasError, c.err)
		} else {
			t.Errorf("Cascade Should Have Error: %v, Cascade has Error: %v", hasError, c.err != nil)
		}

	}
}
