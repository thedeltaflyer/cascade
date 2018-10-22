package cascade

import (
	"context"
)

// WithContext links a Context to a new `RootCascade`. When the provided Context is Cancelled,
// the Cascade will be killed.
//
// The function also returns a child context that will be cancelled when the Cascade is killed or cancelled.
// (Regardless of the state of the parent Context)
func WithContext(ctx context.Context) (*Cascade, context.Context) {
	cas := RootCascade()
	return cas, cas.linkWithContext(ctx)
}

// WithContext links a Context to a new child Cascade. When the provided Context is Cancelled,
// the child Cascade will be killed.
//
// The function also returns a child context that will be cancelled when the Cascade is killed or cancelled.
// (Regardless of the state of the parent Context)
func (c *Cascade) WithContext(ctx context.Context) (*Cascade, context.Context) {
	cas := c.ChildCascade()
	return cas, cas.linkWithContext(ctx)
}

func (c *Cascade) linkWithContext(ctx context.Context) context.Context {
	if ctx.Done() != nil {
		go func() {
			select {
			case <-c.Dying():
			case <-ctx.Done():
				c.Kill()
			}
		}()
	}
	c.muCtx.Lock()
	c.ctx = ctx
	c.muCtx.Unlock()
	tracked, cancel := context.WithCancel(ctx)
	c.linkTrackedContext(ctx, tracked, cancel)
	return tracked
}

// Context returns a `context.Context` that will be cancelled when the Cascade that it was
// generated from is killed or cancelled.
//
// If a Context is provided, it will be used as the parent for the new Context. If `nil` is passed,
// either the Cascade's parent Context (if it exists) or `context.Background()` will
// be used as the parent.
func (c *Cascade) Context(ctx context.Context) context.Context {
	if ctx == nil {
		cc, ret := func() (context.Context, bool) {
			c.muCtx.Lock()
			defer c.muCtx.Unlock()
			if c.ctx == nil {
				c.muCtx.Unlock()
				cc := c.linkWithContext(context.Background())
				c.muCtx.Lock()
				return cc, true
			}
			return c.ctx, false
		}()
		if ret {
			return cc
		}
		ctx = cc
	}

	if c.Alive() {
		cc := func() context.Context {
			c.muCtx.Lock()
			defer c.muCtx.Unlock()
			if child, ok := c.trackedCtx[ctx]; ok {
				select {
				case <-child.context.Done():
					delete(c.trackedCtx, ctx)
				default:
					return child.context
				}
			}
			return nil
		}()
		if cc != nil {
			return cc
		}
	}

	tracked, cancel := context.WithCancel(ctx)
	c.linkTrackedContext(ctx, tracked, cancel)
	return tracked
}

func (c *Cascade) linkTrackedContext(ctx context.Context, child interface{}, cancel func()) {
	// Check to make sure that the cascade hasn't already died!
	if c.IsDead() {
		cancel()
		return
	}

	c.muCtx.Lock()
	c.trackedCtx[ctx] = trackedContext{child.(context.Context), cancel}

	// Double-check that all the other tracked contexts are still ok
	for ctx, tracked := range c.trackedCtx {
		select {
		case <-tracked.context.Done():
			delete(c.trackedCtx, ctx)
		default:
		}
	}
	c.muCtx.Unlock()
}
