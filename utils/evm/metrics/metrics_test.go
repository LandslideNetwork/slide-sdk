package metrics

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

const FANOUT = 128

func BenchmarkMetrics(b *testing.B) {
	r := NewRegistry()
	c := NewRegisteredCounter("counter", r)
	h := NewRegisteredHistogram("histogram", r, NewUniformSample(100))
	m := NewRegisteredMeter("meter", r)
	t := NewRegisteredTimer("timer", r)
	b.ResetTimer()
	ch := make(chan bool)

	wgD := &sync.WaitGroup{}

	wgW := &sync.WaitGroup{}

	wg := &sync.WaitGroup{}
	wg.Add(FANOUT)
	for i := 0; i < FANOUT; i++ {
		go func(i int) {
			defer wg.Done()
			for i := 0; i < b.N; i++ {
				c.Inc(1)
				h.Update(int64(i))
				m.Mark(1)
				t.Update(1)
			}
		}(i)
	}
	wg.Wait()
	close(ch)
	wgD.Wait()
	wgW.Wait()
}

func Example() {
	c := NewCounter()
	Register("money", c)
	c.Inc(17)

	// Threadsafe registration
	t := GetOrRegisterTimer("db.get.latency", nil)
	t.Time(func() { time.Sleep(10 * time.Millisecond) })
	t.Update(1)

	fmt.Println(c.Snapshot().Count())
	fmt.Println(t.Snapshot().Min())
	// Output: 17
	// 1
}
