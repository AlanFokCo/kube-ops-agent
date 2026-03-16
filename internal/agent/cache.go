package agent

import (
	"sync"
	"time"

	"github.com/alanfokco/agentscope-go/pkg/agentscope/agent"
)

// CachedAgent holds cached Agent instance and metadata.
type CachedAgent struct {
	Agent     *agent.ReActAgent
	CreatedAt time.Time
	LastUsed  time.Time
	UseCount  int
}

// AgentCache LRU-caches Agent instances, aligned with Python AgentCache.
type AgentCache struct {
	mu        sync.RWMutex
	cache     map[string]*CachedAgent
	maxSize   int
	ttlSec    int
	idleSec   int
	hits      int
	misses    int
	evictions int
}

// NewAgentCache creates an Agent cache.
func NewAgentCache(maxSize, ttlSec, idleSec int) *AgentCache {
	if maxSize <= 0 {
		maxSize = 10
	}
	if ttlSec <= 0 {
		ttlSec = 3600
	}
	if idleSec <= 0 {
		idleSec = 600
	}
	return &AgentCache{
		cache:   make(map[string]*CachedAgent),
		maxSize: maxSize,
		ttlSec:  ttlSec,
		idleSec: idleSec,
	}
}

// GetOrCreate gets or creates Agent; create is the factory function.
func (c *AgentCache) GetOrCreate(
	key string,
	create func() (*agent.ReActAgent, error),
) (*agent.ReActAgent, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	if ent, ok := c.cache[key]; ok {
		if now.Sub(ent.CreatedAt).Seconds() > float64(c.ttlSec) {
			delete(c.cache, key)
			c.misses++
		} else {
			ent.LastUsed = now
			ent.UseCount++
			c.hits++
			return ent.Agent, nil
		}
	}

	c.misses++
	agent, err := create()
	if err != nil {
		return nil, err
	}
	for len(c.cache) >= c.maxSize {
		c.evictOldestLocked()
	}
	c.cache[key] = &CachedAgent{
		Agent:     agent,
		CreatedAt: now,
		LastUsed:  now,
		UseCount:  1,
	}
	return agent, nil
}

func (c *AgentCache) evictOldestLocked() {
	var oldestKey string
	var oldestTime time.Time
	for k, v := range c.cache {
		if oldestKey == "" || v.LastUsed.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.LastUsed
		}
	}
	if oldestKey != "" {
		delete(c.cache, oldestKey)
		c.evictions++
	}
}

// Remove deletes cache entry by key, aligned with Python AgentCache.
func (c *AgentCache) Remove(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.cache[key]; ok {
		delete(c.cache, key)
		return true
	}
	return false
}

// Clear empties all cache, aligned with Python AgentCache.
func (c *AgentCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]*CachedAgent)
}

// CleanupIdle removes Agents idle for too long.
func (c *AgentCache) CleanupIdle() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	n := 0
	for k, v := range c.cache {
		if now.Sub(v.LastUsed).Seconds() > float64(c.idleSec) {
			delete(c.cache, k)
			n++
		}
	}
	return n
}

// Stats returns cache statistics.
func (c *AgentCache) Stats() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	total := c.hits + c.misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(c.hits) / float64(total) * 100
	}
	return map[string]any{
		"size":                 len(c.cache),
		"max_size":             c.maxSize,
		"hits":                 c.hits,
		"misses":               c.misses,
		"evictions":            c.evictions,
		"hit_rate_percent":     hitRate,
		"ttl_seconds":          c.ttlSec,
		"idle_timeout_seconds": c.idleSec,
	}
}
