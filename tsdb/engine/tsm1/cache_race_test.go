// +build !race

package tsm1_test

import (
	"fmt"
	"github.com/influxdb/influxdb/tsdb/engine/tsm1"
	"math/rand"
	"sync"
	"testing"
	"time"
)

func TestCheckConcurrentReadsAreSafe(t *testing.T) {
	values := make(tsm1.Values, 1000)
	timestamps := make([]time.Time, len(values))
	series := make([]string, 100)
	for i := range timestamps {
		timestamps[i] = time.Unix(int64(rand.Int63n(int64(len(values)))), 0).UTC()
	}

	for i := range values {
		values[i] = tsm1.NewValue(timestamps[i*len(timestamps)/len(values)], float64(i))
	}

	for i := range series {
		series[i] = fmt.Sprintf("series%d", i)
	}

	wg := sync.WaitGroup{}
	c := tsm1.NewCache(1000000)

	ch := make(chan struct{})
	for _, s := range series {
		for _, v := range values {
			c.Write(s, tsm1.Values{v})
		}
		wg.Add(3)
		go func(s string) {
			defer wg.Done()
			<-ch
			c.Values(s)
		}(s)
		go func(s string) {
			defer wg.Done()
			<-ch
			c.Values(s)
		}(s)
		go func(s string) {
			defer wg.Done()
			<-ch
			c.Values(s)
		}(s)
	}
	close(ch)
	wg.Wait()
}

func TestCacheRace(t *testing.T) {
	values := make(tsm1.Values, 1000)
	timestamps := make([]time.Time, len(values))
	series := make([]string, 100)
	for i := range timestamps {
		timestamps[i] = time.Unix(int64(rand.Int63n(int64(len(values)))), 0).UTC()
	}

	for i := range values {
		values[i] = tsm1.NewValue(timestamps[i*len(timestamps)/len(values)], float64(i))
	}

	for i := range series {
		series[i] = fmt.Sprintf("series%d", i)
	}

	wg := sync.WaitGroup{}
	c := tsm1.NewCache(1000000)

	ch := make(chan struct{})
	for _, s := range series {
		for _, v := range values {
			c.Write(s, tsm1.Values{v})
		}
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			<-ch
			c.Values(s)
		}(s)
	}
	wg.Add(1)
	go func() {
		wg.Done()
		<-ch
		s := c.Snapshot()
		s.Deduplicate()
		c.ClearSnapshot()
	}()
	close(ch)
	wg.Wait()
}

func TestCacheRace2Compacters(t *testing.T) {
	values := make(tsm1.Values, 1000)
	timestamps := make([]time.Time, len(values))
	series := make([]string, 100)
	for i := range timestamps {
		timestamps[i] = time.Unix(int64(rand.Int63n(int64(len(values)))), 0).UTC()
	}

	for i := range values {
		values[i] = tsm1.NewValue(timestamps[i*len(timestamps)/len(values)], float64(i))
	}

	for i := range series {
		series[i] = fmt.Sprintf("series%d", i)
	}

	wg := sync.WaitGroup{}
	c := tsm1.NewCache(1000000)

	ch := make(chan struct{})
	for _, s := range series {
		for _, v := range values {
			c.Write(s, tsm1.Values{v})
		}
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			<-ch
			c.Values(s)
		}(s)
	}
	fileCounter := 0
	mapFiles := map[int]bool{}
	mu := sync.Mutex{}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			wg.Done()
			<-ch
			s := c.Snapshot()
			mu.Lock()
			mapFiles[fileCounter] = true
			fileCounter++
			myFiles := map[int]bool{}
			for k, e := range mapFiles {
				myFiles[k] = e
			}
			mu.Unlock()
			s.Deduplicate()
			c.ClearSnapshot()
			mu.Lock()
			defer mu.Unlock()
			for k, _ := range myFiles {
				if _, ok := mapFiles[k]; !ok {
					t.Fatalf("something else deleted one of my files")
				} else {
					delete(mapFiles, k)
				}
			}
		}()
	}
	close(ch)
	wg.Wait()
}
