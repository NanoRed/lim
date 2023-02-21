package container_test

import (
	"log"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/NanoRed/lim/internal/container"
)

func BenchmarkSafePool(b *testing.B) {
	var wg sync.WaitGroup
	var wg2 sync.WaitGroup
	rand.Seed(time.Now().UnixNano())
	pool := container.NewSafePool()
	c := make(chan *container.Object, 1000000)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for obj := range c {
				pool.Remove(obj)
			}
		}()
	}
	for i := 0; i < 10; i++ {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			for j := 0; j < 100000; j++ {
				obj := pool.Add(j)
				if j%2 == 0 {
					c <- obj
				}
			}
		}()
	}
	wg2.Wait()
	close(c)
	wg.Wait()
	count := 0
	for i := 0; i < 1000; i++ {
		count = 0
		for current := pool.Entry(); current != nil; current = current.Next() {
			count++
		}
	}
	log.Println(count)
}
