package main

import (
	"sync"
	"testing"
	"time"
)

func TestTokenBucket_Allow(t *testing.T) {
	rl := NewTokenBucket(10, 5, 5)

	tests := []struct {
		name     string
		tokens   int
		want     bool
		waitTime time.Duration
	}{
		{"Request 1 token, expect allowed", 1, true, 0},
		{"Request 5 tokens, expect denied (exceeds current tokens)", 5, false, 0},
		{"Request 5 tokens after 1 second, expect allowed", 5, true, time.Second},
		{"Request 10 tokens after 2 seconds, expect allowed (tokens replenished)", 10, true, 2 * time.Second},
		{"Request 0 tokens, expect denied (invalid request)", 0, false, 0},
		{"Request -1 tokens, expect denied (invalid request)", -1, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.waitTime > 0 {
				time.Sleep(tt.waitTime)
			}

			got := rl.Allow(tt.tokens)
			if got != tt.want {
				t.Errorf("Allow(%d) = %v, want %v", tt.tokens, got, tt.want)
			}
		})
	}

	rl.Stop()
}

func TestTokenBucket_Stop(t *testing.T) {
	rl := NewTokenBucket(10, 5, 5)
	rl.Stop()

	if rl.Allow(1) {
		t.Error("Allow() should return false after Stop() is called")
	}
}

func TestTokenBucket_Concurrency(t *testing.T) {
	wg := &sync.WaitGroup{}
	rl := NewTokenBucket(10, 5, 10)
	defer rl.Stop()

	tokens := []int{1, 2, 3, 4, 1}
	results := make(chan bool, len(tokens))

	for _, token := range tokens {
		wg.Add(1)
		go func(tk int) {
			defer wg.Done()
			// each goroutine makes a request of tk tokens
			results <- rl.Allow(tk)
		}(token)
	}

	wg.Wait()
	close(results)
	i := 0
	for res := range results {
		if i < len(tokens)-1 {
			if !res {
				t.Errorf("Expected token to be allowed but it was denied")
			}
		} else {
			if res {
				t.Errorf("Expected token to be denied but it was allowed")
			}
		}

		i += 1
	}
}

func TestLeakyBucket_Allow(t *testing.T) {
	rl := NewLeakyBucket(10, 5)

	tests := []struct {
		name     string
		tokens   int
		want     bool
		waitTime time.Duration
	}{
		{"Request 1 token, expect denied(leaky bucket is full)", 1, false, 0},
		{"Request 5 tokens after 1 second, expect allowed (5 tokens got leaked in 1 second)", 5, true, time.Second},
		{"Request 1 token, expect denied(leaky bucket is full)", 1, false, 0},
		{"Request 10 tokens after 2 seconds, expect allowed (leaky bucket should be empty)", 10, true, 2 * time.Second},
		{"Request 15 tokens, expect denied (exceeds bucket capacity)", 15, false, 2 * time.Second},
		{"Request 0 tokens, expect denied (invalid request)", 0, false, 0},
		{"Request -1 tokens, expect denied (invalid request)", -1, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.waitTime > 0 {
				time.Sleep(tt.waitTime)
			}

			got := rl.Allow(tt.tokens)
			if got != tt.want {
				t.Errorf("Allow(%d) = %v, want %v", tt.tokens, got, tt.want)
			}
		})
	}

	rl.Stop()
}

func TestLeakyBucket_Stop(t *testing.T) {
	rl := NewLeakyBucket(10, 5)
	rl.Stop()

	if rl.Allow(1) {
		t.Error("Allow() should return false after Stop() is called")
	}
}

func TestLeakyBucket_Concurrency(t *testing.T) {
	wg := &sync.WaitGroup{}
	rl := NewLeakyBucket(10, 5)
	defer rl.Stop()

	tokens := []int{1, 2, 3, 4, 1}
	results := make(chan bool, len(tokens))

	time.Sleep(2 * time.Second)

	for _, token := range tokens {
		wg.Add(1)
		go func(tk int) {
			defer wg.Done()
			// each goroutine makes a request of tk tokens
			results <- rl.Allow(tk)
		}(token)
	}

	wg.Wait()
	close(results)
	i := 0
	for res := range results {
		if i < len(tokens)-1 {
			if !res {
				t.Errorf("Expected token to be allowed but it was denied")
			}
		} else {
			if res {
				t.Errorf("Expected token to be denied but it was allowed")
			}
		}

		i += 1
	}
}

func TestFixedWindow_Allow(t *testing.T) {
	rl := NewFixedWindow(1, 15)

	tests := []struct {
		name     string
		tokens   int
		want     bool
		waitTime time.Duration
	}{
		{"Request 5 tokens, expect allowed", 5, true, 0},
		{"Request 10 tokens, expect allowed (can grant upto 15 tokens with in a fixed window)", 10, true, 0},
		{"Request 1 token, expect denied (capacity reached with in the current window)", 1, false, 0},
		{"Request 15 tokens, expect allowed (upto 15 tokens allowed with in the new window)", 15, true, 1 * time.Second},
		{"Request 16 tokens, expect denied (exceeds window capacity)", 16, false, 1 * time.Second},
		{"Request 0 tokens, expect denied (invalid request)", 0, false, 0},
		{"Request -1 tokens, expect denied (invalid request)", -1, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.waitTime > 0 {
				time.Sleep(tt.waitTime)
			}

			got := rl.Allow(tt.tokens)
			if got != tt.want {
				t.Errorf("Allow(%d) = %v, want %v", tt.tokens, got, tt.want)
			}
		})
	}

	rl.Stop()
}

func TestFixedWindow_Stop(t *testing.T) {
	rl := NewFixedWindow(1, 10)
	rl.Stop()

	if rl.Allow(1) {
		t.Error("Allow() should return false after Stop() is called")
	}
}

func TestFixedWindow_Concurrency(t *testing.T) {
	wg := &sync.WaitGroup{}
	rl := NewFixedWindow(1, 10)
	numRequests := 10
	results := make([]bool, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			// each goroutine requests index number of tokens
			results[index] = rl.Allow(index)

		}(i)
	}

	wg.Wait()

	// expecting exactly 10 successful requests and the rest to be denied
	successCount := 0
	for _, allowed := range results {
		if allowed {
			successCount++
		}
	}

	if successCount != 4 {
		t.Errorf("Expected 4 successful requests, but got %d", successCount)
	}
	rl.Stop()
}

func TestSlidingWindow_Allow(t *testing.T) {
	rl := NewSlidingWindow(15, 500*time.Millisecond)

	tests := []struct {
		name     string
		tokens   int
		want     bool
		waitTime time.Duration
	}{
		{"Request 5 tokens, expect allowed", 5, true, 0},
		{"Request 10 tokens, expect allowed (can grant upto 15 tokens with in a fixed window)", 10, true, 0},
		{"Request 1 token, expect denied (capacity reached with in the current window)", 1, false, 0},
		{"Request 15 tokens, expect allowed (upto 15 tokens allowed with in the new window)", 15, true, 500 * time.Millisecond},
		{"Request 1 token, expect denied (capacity reached must wait for atleast 500 milliseconds before making any requests)", 1, false, 100 * time.Millisecond},
		{"Request 10 tokens, expect allowed (window has slided)", 10, true, 400 * time.Millisecond},
		{"Request 16 tokens, expect denied (exceeded window capacity)", 16, false, 1000 * time.Millisecond},
		{"Request 0 tokens, expect denied (invalid request)", 0, false, 0},
		{"Request -1 tokens, expect denied (invalid request)", -1, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.waitTime > 0 {
				time.Sleep(tt.waitTime)
			}

			got := rl.Allow(tt.tokens)
			if got != tt.want {
				t.Errorf("Allow(%d) = %v, want %v", tt.tokens, got, tt.want)
			}
		})
	}

	rl.Stop()
}

func TestSlidingWindow_Stop(t *testing.T) {
	rl := NewSlidingWindow(1, 10)
	rl.Stop()

	if rl.Allow(1) {
		t.Error("Allow() should return false after Stop() is called")
	}
}

func TestSlidingWindow_Concurrency(t *testing.T) {
	rl := NewSlidingWindow(10, 500*time.Millisecond)

	var wg sync.WaitGroup
	numRequests := 20
	results := make([]bool, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			// each goroutine requests 1 token
			results[index] = rl.Allow(1)
		}(i)
	}

	wg.Wait()

	// expecting exactly 10 successful requests and the rest to be denied
	successCount := 0
	for _, allowed := range results {
		if allowed {
			successCount++
		}
	}

	if successCount != 10 {
		t.Errorf("Expected 10 successful requests, but got %d", successCount)
	}
	rl.Stop()
}
