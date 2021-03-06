package main

import (
	"encoding/json"
	"runtime"
	"time"
)

// The different MessageTypes for statusCollectorMessage
const (
	StatusTypeCacheHit = iota
	StatusTypeCacheMiss

	StatusTypeRequested
	StatusTypeAPIRequested
	StatusTypeErrored
)

type statusCollectorMessage struct {
	// The type of message this is.
	MessageType uint

	// If MessageType == StatusTypeRequested, StatusTypeAPIRequested or StatusTypeErrored then this is the state we are reporting.
	StatusType string
}

type StatusCollector struct {
	info struct {
		// Number of bytes allocated to the process.
		ImgdMem uint64
		// Time in seconds the process has been running for
		Uptime int64
		// Number of times an error has been recorded.
		Errored map[string]uint
		// Number of times a request type has been requested.
		Requested map[string]uint
		// Number of times an API request type has been made.
		APIRequested map[string]uint
		// Number of times skins have been served from the cache.
		CacheHits uint
		// Number of times skins have failed to be served from the cache.
		CacheMisses uint
		// Number of skins in cache.
		CacheSize uint
		// Size of cache memory.
		CacheMem uint64
	}

	// Unix timestamp the process was booted at.
	StartedAt int64

	// Channel for feeding in input data.
	inputData chan statusCollectorMessage
}

func MakeStatsCollector() *StatusCollector {
	collector := &StatusCollector{}
	collector.StartedAt = time.Now().Unix()
	collector.info.Errored = map[string]uint{}
	collector.info.Requested = map[string]uint{}
	collector.info.APIRequested = map[string]uint{}
	collector.inputData = make(chan statusCollectorMessage, 5)

	// Run a function every five seconds to collect time-based info.
	go func() {
		ticker := time.NewTicker(time.Second * 5)

		for {
			select {
			case <-ticker.C:
				collector.Collect()
			case msg := <-collector.inputData:
				collector.handleMessage(msg)
			}
		}
	}()

	return collector
}

// Message handler function, called inside goroutine.
func (s *StatusCollector) handleMessage(msg statusCollectorMessage) {
	switch msg.MessageType {
	case StatusTypeCacheHit:
		cacheCounter.WithLabelValues("hit").Inc()
		s.info.CacheHits++
	case StatusTypeCacheMiss:
		cacheCounter.WithLabelValues("miss").Inc()
		s.info.CacheMisses++
	case StatusTypeErrored:
		err := msg.StatusType
		errorCounter.WithLabelValues(err).Inc()
		if _, exists := s.info.Errored[err]; exists {
			s.info.Errored[err]++
		} else {
			s.info.Errored[err] = 1
		}
	case StatusTypeRequested:
		req := msg.StatusType
		requestCounter.WithLabelValues(req).Inc()
		if _, exists := s.info.Requested[req]; exists {
			s.info.Requested[req]++
		} else {
			s.info.Requested[req] = 1
		}
	case StatusTypeAPIRequested:
		req := msg.StatusType
		apiCounter.WithLabelValues(req).Inc()
		if _, exists := s.info.APIRequested[req]; exists {
			s.info.APIRequested[req]++
		} else {
			s.info.APIRequested[req] = 1
		}
	}
}

// Encodes the info struct to a JSON string byte slice
func (s *StatusCollector) ToJSON() []byte {
	results, _ := json.Marshal(s.info)
	return results
}

// "cron" function that updates current information
func (s *StatusCollector) Collect() {
	memstats := &runtime.MemStats{}
	runtime.ReadMemStats(memstats)

	s.info.ImgdMem = memstats.Alloc
	s.info.Uptime = time.Now().Unix() - s.StartedAt
	s.info.CacheSize = cache.size()
	s.info.CacheMem = cache.memory()
}

// Increments the error counter for the specific type.
func (s *StatusCollector) Errored(errorType string) {
	s.inputData <- statusCollectorMessage{
		MessageType: StatusTypeErrored,
		StatusType:  errorType,
	}
}

// Increments the request counter for the specific type.
func (s *StatusCollector) Requested(reqType string) {
	s.inputData <- statusCollectorMessage{
		MessageType: StatusTypeRequested,
		StatusType:  reqType,
	}
}

// Increments the request counter for the specific type.
func (s *StatusCollector) APIRequested(reqType string) {
	s.inputData <- statusCollectorMessage{
		MessageType: StatusTypeAPIRequested,
		StatusType:  reqType,
	}
}

// Should be called every time we serve a cached skin.
func (s *StatusCollector) HitCache() {
	s.inputData <- statusCollectorMessage{
		MessageType: StatusTypeCacheHit,
	}
}

// Should be called every time we try and fail to serve a cached skin.
func (s *StatusCollector) MissCache() {
	s.inputData <- statusCollectorMessage{
		MessageType: StatusTypeCacheMiss,
	}
}
