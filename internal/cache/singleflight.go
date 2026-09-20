package cache

import "sync"

// group ensures only one in-flight request per key.
// When multiple goroutines request the same key simultaneously,
// only the first actually loads; the rest share the result.
type group struct {
	mu sync.Mutex
	m  map[string]*call
}

type call struct {
	wg  sync.WaitGroup
	val interface{}
	err error
}

func newGroup() *group {
	return &group{m: make(map[string]*call)}
}

// Do executes fn if no other goroutine is already running the same key.
// The result is shared across all callers for the same key.
func (g *group) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	g.mu.Lock()

	if g.m == nil {
		g.m = make(map[string]*call)
	}

	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}

	c := &call{}
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	c.val, c.err = fn()

	g.mu.Lock()
	c.wg.Done()
	delete(g.m, key)
	g.mu.Unlock()

	return c.val, c.err
}
