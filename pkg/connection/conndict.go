package connection

import (
	"errors"
	"sync"
)

var connDict struct {
	sync.Map // kv container for label -> ConnSet
	sync.Mutex
}

// Label label a connection
func Label(c *Connection, label string) {
	if label == "" {
		return
	}
	defer func() {
		c.lm.Lock()
		defer c.lm.Unlock()
		c.label[label] = struct{}{}
	}()
	if val, ok := connDict.Load(label); ok {
		val.(*ConnSet).add(c)
		return
	}
	connDict.Lock()
	defer connDict.Unlock()
	if val, ok := connDict.Load(label); ok { // double check
		val.(*ConnSet).add(c)
		return
	}
	connSet := newConnSet()
	connSet.add(c)
	connDict.Store(label, connSet)
	return
}

// RemoveLabel remove a label from a connection
func RemoveLabel(c *Connection, label string) {
	defer func() {
		c.lm.Lock()
		defer c.lm.Unlock()
		delete(c.label, label)
	}()
	if val, ok := connDict.Load(label); ok {
		connSet := val.(*ConnSet)
		connSet.remove(c)
		if connSet.len() <= 0 {
			connDict.Delete(label)
		}
	}
}

// ClearLabel remove all the label from a connection
func ClearLabel(c *Connection) {
	c.lm.Lock()
	defer c.lm.Unlock()
	for label := range c.label {
		if val, ok := connDict.Load(label); ok {
			connSet := val.(*ConnSet)
			connSet.remove(c)
			if connSet.len() <= 0 {
				connDict.Delete(label)
			}
		}
		delete(c.label, label)
	}
}

// FindConnSet find a connections set by label
func FindConnSet(label string) (*ConnSet, error) {
	if val, ok := connDict.Load(label); ok {
		return val.(*ConnSet), nil
	}
	return nil, errors.New("can't find corresponding connections set")
}
