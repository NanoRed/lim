package lim

import (
	"bytes"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
)

type group struct {
	connections sync.Map
}

func newGroup() *group {
	return &group{}
}

func (g *group) add(c *connection) {
	g.connections.Store(c, struct{}{})
}

func (g *group) remove(c *connection) {
	g.connections.Delete(c)
}

func (g *group) do(f func(c *connection)) {
	var wg sync.WaitGroup
	g.connections.Range(func(key, value interface{}) bool {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f(key.(*connection))
		}()
		return true
	})
	wg.Wait()
}

var groups struct {
	sync.Map
	sync.Mutex
}

// Label label a connection
func Label(c *connection, label string) {
	c.m.Lock()
	c.label[label] = struct{}{}
	c.m.Unlock()
	if val, ok := groups.Load(label); ok {
		val.(*group).add(c)
		return
	}
	groups.Lock()
	defer groups.Unlock()
	if val, ok := groups.Load(label); ok { // double check
		val.(*group).add(c)
		return
	}
	group := newGroup()
	group.add(c)
	groups.Store(label, group)
	return
}

// Message send a message to label clients
func Message(label string, content []byte) {
	if val, ok := groups.Load(label); ok {
		b := &bytes.Buffer{}
		b.WriteString(fmt.Sprintf("%d,", len(content)))
		b.Write(content)
		val.(*group).do(func(c *connection) {
			if atomic.LoadInt64(&c.status) != 0 {
				n, err := c.Write(b.Bytes())
				if err != nil {
					log.Printf("[Error]io writing error: %v %v %v", label, n, err)
				}
			}
		})
	} else {
		log.Println("[Error]label doesn't have any connections")
	}
	return
}
