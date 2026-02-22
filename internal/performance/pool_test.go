package performance

import (
	"bytes"
	"runtime"
	"sync"
	"testing"
)

func TestBufferPool(t *testing.T) {
	pool := NewBufferPool()

	// Get buffer
	buf := pool.Get()
	if buf == nil {
		t.Fatal("expected non-nil buffer")
	}

	// Use buffer
	buf.WriteString("test data")

	// Return to pool
	pool.Put(buf)

	// Get again - should be reset
	buf2 := pool.Get()
	if buf2.Len() != 0 {
		t.Errorf("expected reset buffer, got len=%d", buf2.Len())
	}
}

func TestBufferPoolConcurrency(t *testing.T) {
	pool := NewBufferPool()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := pool.Get()
			buf.WriteString("concurrent test")
			pool.Put(buf)
		}()
	}

	wg.Wait()
}

func TestRingBuffer(t *testing.T) {
	rb := NewRingBuffer[int](16)

	// Test push/pop
	if !rb.TryPush(1) {
		t.Fatal("expected push to succeed")
	}

	val, ok := rb.TryPop()
	if !ok {
		t.Fatal("expected pop to succeed")
	}
	if val != 1 {
		t.Errorf("expected 1, got %d", val)
	}

	// Test empty
	_, ok = rb.TryPop()
	if ok {
		t.Error("expected pop from empty buffer to fail")
	}

	// Test full
	for i := 0; i < 16; i++ {
		if !rb.TryPush(i) {
			t.Fatalf("expected push %d to succeed", i)
		}
	}

	if rb.TryPush(100) {
		t.Error("expected push to full buffer to fail")
	}
}

func TestRingBufferConcurrency(t *testing.T) {
	rb := NewRingBuffer[int](1024)
	var wg sync.WaitGroup

	// Producer
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			for !rb.TryPush(i) {
				runtime.Gosched()
			}
		}
	}()

	// Consumer
	wg.Add(1)
	go func() {
		defer wg.Done()
		count := 0
		for count < 1000 {
			if _, ok := rb.TryPop(); ok {
				count++
			} else {
				runtime.Gosched()
			}
		}
	}()

	wg.Wait()
}

func TestSlicePool(t *testing.T) {
	pool := NewSlicePool[int](100)

	s := pool.Get()
	if cap(s) != 100 {
		t.Errorf("expected cap=100, got %d", cap(s))
	}

	s = append(s, 1, 2, 3)
	pool.Put(s)

	s2 := pool.Get()
	if len(s2) != 0 {
		t.Errorf("expected len=0 after reset, got %d", len(s2))
	}
}

func TestFastStringBuilder(t *testing.T) {
	b := NewFastStringBuilder()
	b.WriteString("hello")
	b.WriteByte(' ')
	b.WriteString("world")

	result := b.String()
	if result != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", result)
	}

	b.Reset()
	if b.Len() != 0 {
		t.Errorf("expected len=0 after reset, got %d", b.Len())
	}
}

func BenchmarkBufferPool(b *testing.B) {
	pool := NewBufferPool()
	data := []byte("benchmark data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := pool.Get()
		buf.Write(data)
		_ = buf.String()
		pool.Put(buf)
	}
}

func BenchmarkRingBuffer(b *testing.B) {
	rb := NewRingBuffer[int](1024)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rb.TryPush(1)
			rb.TryPop()
		}
	})
}

func BenchmarkFastStringBuilder(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder := NewFastStringBuilder()
		builder.WriteString("hello")
		builder.WriteByte(' ')
		builder.WriteString("world")
		_ = builder.String()
	}
}

func BenchmarkStandardBuffer(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		buf.WriteString("hello")
		buf.WriteByte(' ')
		buf.WriteString("world")
		_ = buf.String()
	}
}
