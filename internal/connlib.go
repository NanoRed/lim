package internal

import (
	"errors"
	"fmt"
	"sync"

	"github.com/NanoRed/lim/pkg/container"
)

var _connlib = &connLibrary{
	connLabel: &sync.Map{},
	labelConn: &sync.Map{},
	gcpool: &sync.Pool{New: func() any {
		return &pool{SyncPool: container.NewSyncPool()}
	}},
}

type pool struct {
	*container.SyncPool
	rwmu sync.RWMutex
}

type connLibrary struct {
	connLabel *sync.Map
	labelConn *sync.Map
	gcpool    *sync.Pool
}

func (c *connLibrary) register(conn *conn) {
	c.connLabel.LoadOrStore(conn, &sync.Map{})
}

func (c *connLibrary) remove(conn *conn) {
	if v, loaded := c.connLabel.LoadAndDelete(conn); loaded {
		v.(*sync.Map).Range(func(label, node interface{}) bool {
			if p, ok := c.labelConn.Load(label); ok {
				pool := p.(*pool)
				pool.Remove(node.(*container.SyncPoolNode))
				if pool.Entry() == nil {
					pool.rwmu.Lock()
					if pool.Entry() == nil {
						if p, ok := c.labelConn.Load(label); ok && p == pool {
							c.labelConn.Delete(label)
						}
						// c.labelConn.CompareAndDelete(label, pool)
						pool.rwmu.Unlock()
						c.gcpool.Put(pool)
					} else {
						pool.rwmu.Unlock()
					}
				}
			}
			return true
		})
		conn.Close()
	}
}

func (c *connLibrary) label(conn *conn, label string) error {
	v, ok := c.connLabel.Load(conn)
	if !ok {
		return errors.New("connection has been removed")
	}
	smap := v.(*sync.Map)
	if _, ok := smap.Load(label); ok {
		return errors.New("connection label has been existed")
	}
GETPOOL:
	p := c.gcpool.Get()
	v, loaded := c.labelConn.LoadOrStore(label, p)
	if loaded {
		c.gcpool.Put(p)
	}
	pool := v.(*pool)
	pool.rwmu.RLock()
	if v2, _ := c.labelConn.Load(label); v != v2 {
		pool.rwmu.RUnlock()
		goto GETPOOL
	}
	nnode := pool.Add(conn)
	pool.rwmu.RUnlock()
	smap.Store(label, nnode)
	// connection valid check
	if _, ok = c.connLabel.Load(conn); !ok {
		pool.Remove(nnode)
		if pool.Entry() == nil {
			pool.rwmu.Lock()
			if pool.Entry() == nil {
				if p, ok := c.labelConn.Load(label); ok && p == pool {
					c.labelConn.Delete(label)
				}
				// c.labelConn.CompareAndDelete(label, pool)
				pool.rwmu.Unlock()
				c.gcpool.Put(pool)
			} else {
				pool.rwmu.Unlock()
			}
		}
		return errors.New("connection has been removed")
	}
	return nil
}

func (c *connLibrary) dislabel(conn *conn, label string) error {
	v, ok := c.connLabel.Load(conn)
	if !ok {
		return errors.New("connection has been removed")
	}
	smap := v.(*sync.Map)
	node, ok := smap.Load(label)
	if !ok {
		return errors.New("connection label does not exist")
	}
	if p, ok := c.labelConn.Load(label); ok {
		pool := p.(*pool)
		pool.Remove(node.(*container.SyncPoolNode))
		if pool.Entry() == nil {
			pool.rwmu.Lock()
			if pool.Entry() == nil {
				if p, ok := c.labelConn.Load(label); ok && p == pool {
					c.labelConn.Delete(label)
				}
				// c.labelConn.CompareAndDelete(label, pool)
				pool.rwmu.Unlock()
				c.gcpool.Put(pool)
			} else {
				pool.rwmu.Unlock()
			}
		}
	}
	return nil
}

func (c *connLibrary) pool(label string) (*pool, error) {
	if p, ok := c.labelConn.Load(label); ok {
		return p.(*pool), nil
	}
	return nil, fmt.Errorf("failed to get corresponding connection pool: %s", label)
}
