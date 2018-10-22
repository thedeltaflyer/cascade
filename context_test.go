package cascade

import (
	"context"
	"testing"
	"time"
)

func TestWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())

	cas, ctx1 := WithContext(ctx)

	verifyCascadeEndState(t, cas, false, 0, false, 0, true, 1, false)

	select {
	case <-ctx.Done():
		t.Error("_WithContext: Root Context was canceled early!")
	default:
	}
	select {
	case <-ctx1.Done():
		t.Error("_WithContext: Context1 was canceled early!")
	default:
	}

	cas1, ctx2 := WithContext(ctx1)

	verifyCascadeEndState(t, cas1, false, 0, false, 0, true, 1, false)

	select {
	case <-ctx2.Done():
		t.Error("_WithContext: Context2 was canceled early!")
	default:
	}

	cancel()

	okCas := didExitBeforeTime(cas, 1*time.Second)
	okCas1 := didExitBeforeTime(cas1, 1*time.Second)
	if !okCas {
		t.Error("_WithContext: Cas got stuck!")
	}
	if !okCas1 {
		t.Error("_WithContext: Cas1 got stuck!")
	}

	select {
	case <-ctx.Done():
	default:
		t.Error("_WithContext: Root Context was not Cancelled!")
	}
	select {
	case <-ctx1.Done():
	default:
		t.Error("_WithContext: Context1 was not Cancelled!")
	}
	select {
	case <-ctx2.Done():
	default:
		t.Error("_WithContext: Context2 was not Cancelled!")
	}

	verifyCascadeEndState(t, cas1, false, 0, true, 0, true, 0, false)

}

func TestCascade_WithContext(t *testing.T) {
	cas := RootCascade()
	ctx, cancel := context.WithCancel(context.TODO())
	cas1, ctx1 := cas.WithContext(ctx)

	verifyCascadeEndState(t, cas, false, 1, false, 0, false, 0, false)
	verifyCascadeEndState(t, cas1, true, 0, false, 0, true, 1, false)

	select {
	case <-ctx.Done():
		t.Error("WithContext: Root Context was canceled early!")
	default:
	}
	select {
	case <-ctx1.Done():
		t.Error("WithContext: Context1 was canceled early!")
	default:
	}

	cancel()

	okCas1 := didExitBeforeTime(cas1, 1*time.Second)
	if !okCas1 {
		t.Error("WithContext: Cas1 got stuck!")
	}

	select {
	case <-ctx.Done():
	default:
		t.Error("WithContext: Root Context was not Cancelled!")
	}
	select {
	case <-ctx1.Done():
	default:
		t.Error("WithContext: Context1 was not Cancelled!")
	}

	verifyCascadeEndState(t, cas1, true, 0, true, 0, true, 0, false)

	go cas.Kill()

	okCas := didExitBeforeTime(cas, 1*time.Second)
	if !okCas {
		t.Error("WithContext: Cas got stuck!")
	}

	verifyCascadeEndState(t, cas, false, 0, true, 0, false, 0, false)
}

func TestCascade_Context(t *testing.T) {
	cas := RootCascade()
	ctx := context.TODO()

	verifyCascadeEndState(t, cas, false, 0, false, 0, false, 0, false)

	ctx1 := cas.Context(ctx)
	verifyCascadeEndState(t, cas, false, 0, false, 0, false, 1, false)

	ctx2 := cas.Context(nil)
	verifyCascadeEndState(t, cas, false, 0, false, 0, true, 2, false)

	_ = cas.Context(nil)

	select {
	case <-ctx.Done():
		t.Error("Context: Context Cancelled!")
	default:
	}
	select {
	case <-ctx1.Done():
		t.Error("Context: Context1 Cancelled!")
	default:
	}
	select {
	case <-ctx2.Done():
		t.Error("Context: Context2 Cancelled!")
	default:
	}

	go cas.Kill()
	okCas := didExitBeforeTime(cas, 1*time.Second)
	if !okCas {
		t.Error("Context: Cas got stuck!")
	}

	select {
	case <-ctx.Done():
		t.Error("Context: Context Cancelled!")
	case <-time.After(time.Second / 2):
	}
	select {
	case <-ctx1.Done():
	case <-time.After(time.Second / 2):
		t.Error("Context: Context1 Not Cancelled!")
	}
	select {
	case <-ctx2.Done():
	case <-time.After(time.Second / 2):
		t.Error("Context: Context2 Not Cancelled!")
	}

	verifyCascadeEndState(t, cas, false, 0, true, 0, true, 0, false)
}

func TestCascade_ContextCancelFromContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	cas, _ := WithContext(ctx)

	verifyCascadeEndState(t, cas, false, 0, false, 0, true, 1, false)

	cancel()

	okCas := didExitBeforeTime(cas, 1*time.Second)
	if !okCas {
		t.Error("ContextCancelFromContext: Cas got stuck!")
	}

	verifyCascadeEndState(t, cas, false, 0, true, 0, true, 0, false)
}

func TestCascade_ContextCancelFromCascade(t *testing.T) {
	ctx := context.TODO()
	cas, ctx1 := WithContext(ctx)

	verifyCascadeEndState(t, cas, false, 0, false, 0, true, 1, false)

	go cas.Kill()

	okCas := didExitBeforeTime(cas, 1*time.Second)
	if !okCas {
		t.Error("ContextCancelFromCascade: Cas got stuck!")
	}

	verifyCascadeEndState(t, cas, false, 0, true, 0, true, 0, false)

	select {
	case <-ctx.Done():
		t.Error("ContextCancelFromCascade: Context did Cancel!")
	case <-time.After(time.Second / 2):
	}
	select {
	case <-ctx1.Done():
	case <-time.After(time.Second / 2):
		t.Error("ContextCancelFromCascade: Context1 did not Cancel!")
	}
}

func TestCascade_ContextFromKilledCascade(t *testing.T) {
	cas := RootCascade()
	ctx := context.TODO()
	ctxAlt := context.WithValue(ctx, "key", "val") // We need an alt context to force cleanup of stale contexts

	_ = cas.Context(ctx)

	cas.muCtx.Lock()
	ctx1Cancel := cas.trackedCtx[ctx].cancel
	cas.muCtx.Unlock()
	ctx1Cancel()

	_ = cas.Context(ctx)
	_ = cas.Context(ctxAlt)
	_ = cas.Context(ctx)

	cas.muCtx.Lock()
	ctx1Cancel = cas.trackedCtx[ctx].cancel
	ctx2Cancel := cas.trackedCtx[ctxAlt].cancel
	cas.muCtx.Unlock()
	ctx1Cancel()
	ctx2Cancel()

	_ = cas.Context(ctxAlt)

	go cas.Kill()

	okCas := didExitBeforeTime(cas, 1*time.Second)
	if !okCas {
		t.Error("ContextCancelFromKilledCascade: Cas got stuck!")
	}
	ctx2 := cas.Context(ctx)
	select {
	case <-ctx2.Done():
	case <-time.After(time.Second / 2):
		t.Error("ContextCancelFromKilledCascade: Context2 did not Cancel!")
	}
}
