# ratelimitters

Rate limiters are essential for controlling the amount of incoming requests to a service. This project implements various rate-limiting algorithms in Go, including Token Bucket, Leaky Bucket, Fixed Window, and Sliding Window algorithms.

## Table of Contents

- [Installation](#installation)
- [Usage](#usage)
- [Algorithms](#algorithms)
  - [Token Bucket](#token-bucket)
  - [Leaky Bucket](#leaky-bucket)
  - [Fixed Window](#fixed-window)
  - [Sliding Window](#sliding-window)

## Installation

To run this project, you need to have Go installed on your machine.

Clone the repository:

```bash
git clone https://github.com/legosandorigami/ratelimitters.git
cd ratelimitters
```

## Usage

You can create instances of different rate limiters and make requests as shown below:

```go
package main

import "time"

func main() {
    // Example usage of Leaky Bucket algorithm
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
}
```

## Algorithms

### Token Bucket

The Token Bucket algorithm allows a certain number of tokens to be accumulated. Tokens are added at a specified rate, and requests can be fulfilled if there are enough tokens available.

```go
rl := NewTokenBucket(capacity, tokensPerSecond, initialTokens)
```

### Leaky Bucket

The Leaky Bucket algorithm allows requests to be processed at a steady rate. Tokens leak out of the bucket at a defined rate, and if the bucket is full, incoming requests are denied.

```go
rl := NewLeakyBucket(capacity, leakRate)
```

### Fixed Window

The Fixed Window algorithm allows a fixed number of requests in a specified time frame. After the time window expires, the count resets.

```go
rl := NewFixedWindow(windowSize, capacity)
```

### Sliding Window

The Sliding Window algorithm keeps track of the timestamps of requests within a given time frame, allowing for a more flexible rate limiting.

```go
rl := NewSlidingWindow(limit, windowSize)
```