// TODO: scanner, and try to work with bytes only
//
// data:
//
// Tamale;27.5
// Bergen;9.6
// Lodwar;37.1
// Whitehorse;-3.8
// Ouarzazate;19.1
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/exp/maps"
)

// Measurements, as there is no need to keep all numbers around, we can compute
// them on the fly.
type Measurements struct {
	Min   float64
	Max   float64
	Sum   float64
	Count int
}

var bSemi = []byte(";")

func (m *Measurements) Add(v float64) {
	if v > m.Max {
		m.Max = v
	} else if v < m.Min {
		m.Min = v
	}
	m.Sum = m.Sum + v
	m.Count++
}

func (m *Measurements) Merge(o *Measurements) {
	if o.Min < m.Min {
		m.Min = o.Min
	}
	if o.Max > m.Max {
		m.Max = o.Max
	}
	m.Sum = m.Sum + o.Sum
	m.Count = m.Count + o.Count
}

func worker(queue chan [][]byte, result chan map[string]*Measurements, wg *sync.WaitGroup) {
	defer wg.Done()
	var data = make(map[string]*Measurements)
	for batch := range queue {
		for _, line := range batch {
			// find ";"
			index := bytes.Index(line, bSemi)
			if index == -1 {
				log.Fatalf("expected a semicolor: %v", string(line))
			}
			temp, err := strconv.ParseFloat(strings.TrimSpace(string(line[index+1:])), 64)
			if err != nil {
				log.Fatalf("invalid temp: %f", string(line[index+1:]))
			}
			name := string(line[:index])
			if _, ok := data[name]; !ok {
				data[name] = &Measurements{
					Min:   temp,
					Max:   temp,
					Sum:   temp,
					Count: 1,
				}
			} else {
				data[name].Add(temp)
			}
		}
	}
	result <- data
}

func merger(data map[string]*Measurements, result chan map[string]*Measurements, done chan bool) {
	for m := range result {
		for k, v := range m {
			if _, ok := data[k]; !ok {
				data[k] = &Measurements{
					Min:   v.Min,
					Max:   v.Max,
					Sum:   v.Sum,
					Count: v.Count,
				}
			} else {
				data[k].Merge(v)
			}
		}
	}
	done <- true
}

var cpuprofile = flag.String("cpuprofile", "", "file to write cpu profile to")

func main() {
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	var (
		batchSize = 20_000_000
		queue     = make(chan [][]byte)
		result    = make(chan map[string]*Measurements)
		wg        sync.WaitGroup
		done      = make(chan bool)
		// accumulate all results here
		data = make(map[string]*Measurements)
	)
	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go worker(queue, result, &wg)
	}
	go merger(data, result, done)
	// start reading the file and fan out
	scanner := bufio.NewScanner(os.Stdin)
	batch := make([][]byte, batchSize)
	i := 0
	for scanner.Scan() {
		line := scanner.Bytes()
		copy(batch[i], line)
		i++
		if i%batchSize == 0 {
			queue <- batch
			batch = make([][]byte, batchSize)
		}
	}
	if scanner.Err() != nil {
		log.Fatal(scanner.Err())
	}
	queue <- batch[:i] // rest, no copy required
	close(queue)
	wg.Wait()
	close(result)
	<-done
	// At this point, data contains the merged data from all measurements.
	keys := maps.Keys(data)
	sort.Strings(keys)
	for _, k := range keys {
		avg := data[k].Sum / float64(data[k].Count)
		fmt.Printf("%s\t%0.2f/%0.2f/%0.2f\n", k, data[k].Min, data[k].Max, avg)
	}
}
