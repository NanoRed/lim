package connection

import (
	"errors"
	"net"
	"sync"
	"time"

	"github.com/RedAFD/lim/internal/container"
)

type connManager struct {
	p1           sync.Pool
	p2           sync.Pool
	connExtra    *container.SafeMap
	labeledPools *container.SafeMap
}

type connExtra struct {
	m      sync.Mutex
	labels *container.SafeMap
}

const maxLabels = 10

var cm *connManager

func init() {
	cm = &connManager{
		p1:           sync.Pool{New: func() interface{} { return container.NewSafePool() }},
		p2:           sync.Pool{New: func() interface{} { return &connExtra{labels: container.NewSafeMap()} }},
		connExtra:    container.NewSafeMap(),
		labeledPools: container.NewSafeMap(),
	}
}

// Register register a connection
func Register(c net.Conn) {
	cm.connExtra.Store(c, cm.p2.Get().(*connExtra))
}

// Close Deregister a connection
func Close(c net.Conn) {
	if tmp, ok := cm.connExtra.Load(c); ok {
		extra := tmp.(*connExtra)
		extra.m.Lock()
		defer extra.m.Unlock()
		if _, ok := cm.connExtra.Load(c); !ok {
			return
		}

		extra.labels.Range(func(label, obj interface{}) bool {
			extra.labels.Delete(label)
			if pool, ok := cm.labeledPools.Load(label); ok {
				pool.(*container.SafePool).Remove(obj.(*container.Object))
			}
			return true
		})
		cm.p2.Put(extra)
		cm.connExtra.Delete(c)
		c.Close()
	}
}

// Write safety write
func Write(c net.Conn, timeout time.Duration, b []byte) (n int, err error) {
	if tmp, ok := cm.connExtra.Load(c); ok {
		extra := tmp.(*connExtra)
		extra.m.Lock()
		defer extra.m.Unlock()
		if _, ok := cm.connExtra.Load(c); !ok {
			err = errors.New("the connection has been disconnected")
			return
		}
	} else {
		err = errors.New("the connection has been disconnected")
		return
	}

	if err = c.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return
	}
	return c.Write(b)
}

// Label label a connection
func Label(c net.Conn, label string) error {
	var extra *connExtra
	if tmp, ok := cm.connExtra.Load(c); ok {
		extra = tmp.(*connExtra)
		extra.m.Lock()
		defer extra.m.Unlock()
		if _, ok := cm.connExtra.Load(c); !ok {
			return errors.New("the connection has been disconnected")
		}
	} else {
		return errors.New("the connection has been disconnected")
	}

	if _, ok := extra.labels.Load(label); ok {
		return errors.New("label has been exists")
	} else if extra.labels.Count() >= maxLabels {
		return errors.New("the number of labels has exceeded the maximum")
	}
	p := cm.p1.Get()
	pool, loaded := cm.labeledPools.LoadOrStore(label, p)
	if loaded {
		cm.p1.Put(p)
	}
	extra.labels.Store(label, pool.(*container.SafePool).Add(c))

	return nil
}

// Dislabel remove a label from a connection
func Dislabel(c net.Conn, label string) error {
	var extra *connExtra
	if tmp, ok := cm.connExtra.Load(c); ok {
		extra = tmp.(*connExtra)
		extra.m.Lock()
		defer extra.m.Unlock()
		if _, ok := cm.connExtra.Load(c); !ok {
			return errors.New("the connection has been disconnected")
		}
	} else {
		return errors.New("the connection has been disconnected")
	}

	if obj, ok := extra.labels.Load(label); ok {
		extra.labels.Delete(label)
		if pool, ok := cm.labeledPools.Load(label); ok {
			pool.(*container.SafePool).Remove(obj.(*container.Object))
		}
	} else {
		return errors.New("label not exists")
	}

	return nil
}

// FindPool find a connections pool by label
func FindPool(label string) (*container.SafePool, error) {
	if pool, ok := cm.labeledPools.Load(label); ok {
		return pool.(*container.SafePool), nil
	}
	return nil, errors.New("cannot find corresponding pool")
}
