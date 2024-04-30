// TODO: use bufio.Scanner instead of br.ReadString // which allocates
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

func worker(queue chan []string, result chan map[string]*Measurements, wg *sync.WaitGroup) {
	defer wg.Done()
	var data = make(map[string]*Measurements)
	var index int
	for batch := range queue {
		for _, line := range batch {
			index = strings.Index(line, ";")
			if index == -1 {
				log.Fatalf("expected a semicolon: %v", line)
			}
			name := strings.TrimSpace(line[:index])
			temp, err := strconv.ParseFloat(strings.TrimSpace(line[index+1:]), 64)
			if err != nil {
				log.Fatalf("invalid temp: %f", line[index+1:])
			}
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
		queue     = make(chan []string)
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
	batch := make([]string, 0)
	i := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		batch = append(batch, line)
		i++
		if i%batchSize == 0 {
			b := make([]string, len(batch))
			copy(b, batch)
			queue <- b
			batch = nil
		}
	}
	if scanner.Err() != nil {
		log.Fatal(scanner.Err())
	}
	queue <- batch // rest, no copy required
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
