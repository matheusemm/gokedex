package pokecache

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestAddGet(t *testing.T) {
	cases := []struct {
		key string
		val []byte
	}{
		{key: "https://example.com", val: []byte("hello")},
		{key: "https://example.com/path", val: []byte("world")},
		{key: "", val: []byte{}},
	}

	cache := NewCache(5 * time.Second)
	for i, c := range cases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			cache.Add(c.key, c.val)
			got, ok := cache.Get(c.key)
			if !ok {
				t.Fatalf("expected key %q to be present", c.key)
			}
			if !bytes.Equal(got, c.val) {
				t.Errorf("got %q, want %q", got, c.val)
			}
		})
	}
}

func TestGetMissing(t *testing.T) {
	cache := NewCache(5 * time.Second)
	if got, ok := cache.Get("missing"); ok {
		t.Errorf("expected miss, got %q", got)
	}
}

func TestAddOverwrites(t *testing.T) {
	cache := NewCache(5 * time.Second)
	cache.Add("k", []byte("first"))
	cache.Add("k", []byte("second"))

	got, ok := cache.Get("k")
	if !ok {
		t.Fatal("expected key to be present")
	}
	if !bytes.Equal(got, []byte("second")) {
		t.Errorf("got %q, want %q", got, "second")
	}
}

func TestReap(t *testing.T) {
	const ttl = 50 * time.Millisecond
	cache := NewCache(ttl)

	cache.Add("k", []byte("v"))
	if _, ok := cache.Get("k"); !ok {
		t.Fatal("expected key to be present immediately after Add")
	}

	time.Sleep(3 * ttl)

	if got, ok := cache.Get("k"); ok {
		t.Errorf("expected key to be reaped, got %q", got)
	}
}

func TestConcurrentAccess(t *testing.T) {
	cache := NewCache(5 * time.Second)

	const goroutines = 50
	const perGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := range goroutines {
		go func(g int) {
			defer wg.Done()
			for i := range perGoroutine {
				key := fmt.Sprintf("g%d-k%d", g, i)
				cache.Add(key, []byte(key))
				if got, ok := cache.Get(key); !ok || string(got) != key {
					t.Errorf("get %q: ok=%v got=%q", key, ok, got)
					return
				}
			}
		}(g)
	}
	wg.Wait()
}
