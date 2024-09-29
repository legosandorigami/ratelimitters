package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const LIMITER_CAPACITY = 1024

type RateLimiter interface {
	Allow(int) bool
	Stop()
}

type requestTokensCh struct {
	tokens int
	resCh  chan bool
}

type RateLimiterBase struct {
	allowCh  chan requestTokensCh
	stopFunc context.CancelFunc
	wg       sync.WaitGroup
	isClosed bool
	mu       sync.RWMutex
}

func (rlb *RateLimiterBase) Allow(tokens int) bool {
	if tokens <= 0 {
		return false
	}
	isClosed := false
	rlb.mu.RLock()
	isClosed = rlb.isClosed
	rlb.mu.RUnlock()
	if isClosed {
		return false
	}

	reqTokensCh := requestTokensCh{
		tokens: tokens,
		resCh:  make(chan bool, 1),
	}

	rlb.allowCh <- reqTokensCh
	return <-reqTokensCh.resCh
}

func (rlb *RateLimiterBase) Stop() {
	rlb.stopFunc()
	rlb.wg.Wait()
	rlb.mu.Lock()
	rlb.isClosed = true
	rlb.mu.Unlock()
	close(rlb.allowCh)
}

type TokenBucket struct {
	capacity        int
	tokensPerSecond int
	tokens          int
	lastTime        time.Time
	*RateLimiterBase
}

func NewTokenBucket(capacity, tokensPerSecond, tokens int) RateLimiter {
	allowCh := make(chan requestTokensCh, LIMITER_CAPACITY)
	ctx, cancelFunc := context.WithCancel(context.Background())
	rlBase := &RateLimiterBase{
		allowCh:  allowCh,
		stopFunc: cancelFunc,
	}
	rl := &TokenBucket{
		RateLimiterBase: rlBase,
		capacity:        capacity,
		tokensPerSecond: tokensPerSecond,
		tokens:          tokens,
		lastTime:        time.Now(),
	}

	rl.wg.Add(1)
	go rl.tokenBucketAlgorithm(ctx)

	return rl
}

func (rl *TokenBucket) tokenBucketAlgorithm(ctx context.Context) {
	// runs the token bucket algorithm in a separate goroutine and also checks for event(cancelling the context) to stop this goroutine
	defer rl.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case reqTokensCh := <-rl.allowCh:
			fmt.Printf("tokens requested: %d, available tokens: %d ", reqTokensCh.tokens, rl.tokens)

			currentTime := time.Now()
			timePassed := currentTime.Sub(rl.lastTime).Seconds()
			temp := rl.tokens + int(timePassed)*rl.tokensPerSecond
			fmt.Printf("total new tokens: %d ", temp)
			if rl.capacity <= temp {
				rl.tokens = rl.capacity
			} else {
				rl.tokens = temp
			}
			rl.lastTime = currentTime
			resp := false

			if reqTokensCh.tokens <= rl.tokens {
				rl.tokens -= reqTokensCh.tokens
				resp = true
			} else {
				resp = false
			}
			reqTokensCh.resCh <- resp
			close(reqTokensCh.resCh)
		}
	}
}

type LeakyBucket struct {
	capacity int
	leakRate int
	tokens   int
	lastTime time.Time
	*RateLimiterBase
}

func NewLeakyBucket(capacity, leakRate int) RateLimiter {
	allowCh := make(chan requestTokensCh, LIMITER_CAPACITY)
	ctx, cancelFunc := context.WithCancel(context.Background())
	rlBase := &RateLimiterBase{
		allowCh:  allowCh,
		stopFunc: cancelFunc,
	}
	rl := &LeakyBucket{
		RateLimiterBase: rlBase,
		capacity:        capacity,
		leakRate:        leakRate,
		tokens:          capacity,
		lastTime:        time.Now(),
	}

	rl.wg.Add(1)
	go rl.leakyBucketAlgorithm(ctx)

	return rl
}

func (rl *LeakyBucket) leakyBucketAlgorithm(ctx context.Context) {
	// runs the leaky bucket algorithm in a separate goroutine and also checks for event(cancelling the context) to stop this goroutine

	defer rl.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case reqTokensCh := <-rl.allowCh:
			fmt.Printf("tokens requested: %d, available tokens: %d ", reqTokensCh.tokens, rl.tokens)
			currentTime := time.Now()
			timePassed := currentTime.Sub(rl.lastTime).Seconds()

			leakedTokens := int(timePassed) * rl.leakRate

			temp := rl.tokens - leakedTokens
			if temp < 0 {
				rl.tokens = 0
			} else {
				rl.tokens = temp
			}
			fmt.Printf("total new tokens: %d ", rl.tokens)

			rl.lastTime = currentTime
			resp := false

			if reqTokensCh.tokens <= (rl.capacity - rl.tokens) {
				rl.tokens += reqTokensCh.tokens
				resp = true
			} else {
				resp = false
			}
			reqTokensCh.resCh <- resp
			close(reqTokensCh.resCh)
		}
	}
}

type FixedWindow struct {
	tokens     int
	windowSize int
	capacity   int
	lastTime   time.Time
	*RateLimiterBase
}

func NewFixedWindow(windowSize, capacity int) RateLimiter {
	allowCh := make(chan requestTokensCh, LIMITER_CAPACITY)
	ctx, cancelFunc := context.WithCancel(context.Background())
	rlBase := &RateLimiterBase{
		allowCh:  allowCh,
		stopFunc: cancelFunc,
	}
	rl := &FixedWindow{
		RateLimiterBase: rlBase,
		tokens:          capacity,
		capacity:        capacity,
		windowSize:      windowSize,
		lastTime:        time.Now(),
	}

	rl.wg.Add(1)
	go rl.fixedWindowAlgorithm(ctx)

	return rl
}

func (rl *FixedWindow) fixedWindowAlgorithm(ctx context.Context) {
	// runs the fixed window algorithm in a separate goroutine and also checks for event(cancelling the context) to stop this goroutine

	defer rl.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case reqTokensCh := <-rl.allowCh:
			fmt.Printf("tokens requested: %d, available tokens: %d ", reqTokensCh.tokens, rl.tokens)
			currentTime := time.Now()
			timePassed := int(currentTime.Sub(rl.lastTime).Seconds())

			resp := false
			if timePassed >= rl.windowSize {
				rl.lastTime = currentTime
				rl.tokens = rl.capacity - reqTokensCh.tokens
				if rl.tokens < 0 {
					rl.tokens = rl.capacity
					resp = false
				} else {
					resp = true
				}
			} else {
				if rl.tokens >= reqTokensCh.tokens {
					rl.tokens -= reqTokensCh.tokens
					resp = true
				} else {
					resp = false
				}
			}
			fmt.Printf("total new tokens: %d ", rl.tokens)
			reqTokensCh.resCh <- resp
			close(reqTokensCh.resCh)
		}
	}
}

type SlidingWindow struct {
	limit      int
	windowSize time.Duration
	timeStamps []time.Time
	*RateLimiterBase
}

func NewSlidingWindow(limit int, windowSize time.Duration) RateLimiter {
	allowCh := make(chan requestTokensCh, LIMITER_CAPACITY)
	ctx, cancelFunc := context.WithCancel(context.Background())
	rlBase := &RateLimiterBase{
		allowCh:  allowCh,
		stopFunc: cancelFunc,
	}
	rl := &SlidingWindow{
		RateLimiterBase: rlBase,
		limit:           limit,
		windowSize:      windowSize,
		timeStamps:      make([]time.Time, 0),
	}

	rl.wg.Add(1)
	go rl.slidingWindowAlgorithm(ctx)

	return rl
}

func (rl *SlidingWindow) slidingWindowAlgorithm(ctx context.Context) {
	// runs the sliding window algorithm in a separate goroutine and also checks for event(cancelling the context) to stop this goroutine

	defer rl.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case reqTokensCh := <-rl.allowCh:
			currentTime := time.Now()
			// append as many entries as tokens requested
			for i := 0; i < reqTokensCh.tokens; i++ {
				rl.timeStamps = append(rl.timeStamps, currentTime)
			}
			fmt.Printf("total requests: %d, limit: %d ", reqTokensCh.tokens, rl.limit)

			for len(rl.timeStamps) > 0 && rl.timeStamps[0].Before(currentTime.Add(-rl.windowSize)) {
				rl.timeStamps = rl.timeStamps[1:]
			}
			fmt.Printf("total requests after sliding: %d ", len(rl.timeStamps))
			resp := false
			totalTokensInWindow := len(rl.timeStamps)
			if totalTokensInWindow <= rl.limit {
				resp = true
			} else {
				// roll back the tokens if the request can't be fulfilled
				rl.timeStamps = rl.timeStamps[:totalTokensInWindow-reqTokensCh.tokens]
			}
			reqTokensCh.resCh <- resp
			close(reqTokensCh.resCh)
		}
	}
}

func main() {
	// rl := NewTokenBucket(10, 5, 5)
	// var ok bool
	// for i := 0; i < 10; i++ {
	// 	ok = rl.Allow(1)
	// 	if ok {
	// 		fmt.Println("access granted")
	// 	} else {
	// 		fmt.Println("access denied")
	// 		time.Sleep(1 * time.Second)
	// 	}

	// }
	// rl.Stop()

	rl := NewLeakyBucket(10, 200)
	var ok bool
	for i := 0; i < 10; i++ {
		ok = rl.Allow(2)
		if ok {
			fmt.Println("access granted")
			time.Sleep(5 * time.Second)
		} else {
			fmt.Println("access denied")
			time.Sleep(3 * time.Second)
		}

	}
	rl.Stop()

	// rl := NewFixedWindow(1, 5)
	// var ok bool
	// for i := 0; i < 10; i++ {
	// 	ok = rl.Allow(1)
	// 	if ok {
	// 		fmt.Println("access granted")
	// 	} else {
	// 		fmt.Println("access denied")
	// 		time.Sleep(1 * time.Second)
	// 	}

	// }
	// rl.Stop()

	// rl := NewSlidingWindow(5, 1)
	// var ok bool
	// for i := 0; i < 10; i++ {
	// 	ok = rl.Allow(1)
	// 	if ok {
	// 		fmt.Println("access granted")
	// 	} else {
	// 		fmt.Println("access denied")
	// 		time.Sleep(1 * time.Second)
	// 	}

	// }
	// rl.Stop()
}
