package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"sort"
	"strconv"
	"sync"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/mmap"
)

const chunkSize = 67108864

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

// aggregate aggregates measurements by reading a chunk from an io.ReaderAt and
// passing the result to a results channel.
func aggregate(rat io.ReaderAt, offset, length int, resultC chan map[string]*Measurements, sem chan bool, wg *sync.WaitGroup) {
	defer wg.Done()
	if length == 0 {
		return
	}
	buf := make([]byte, length)
	_, err := rat.ReadAt(buf, int64(offset))
	if err == io.EOF {
		return
	}
	if err != nil {
		log.Fatal(err)
	}
	log.Println(offset, length)
	var (
		data    = make(map[string]*Measurements)
		j, k, l = 0, 0, 0 // j=start, k=semi, l=newline
		n       = 0
	)
	for i := 0; i < length; i++ {
		if buf[i] == ';' {
			k = i
		} else if buf[i] == '\n' {
			l = i
			name := string(buf[j:k])
			temp, err := strconv.ParseFloat(string(buf[k+1:l]), 64)
			if err != nil {
				log.Fatal(err)
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
			j = l + 1
			n++
		}
	}
	resultC <- data
	<-sem
}

func merger(data map[string]*Measurements, resultC chan map[string]*Measurements, done chan bool) {
	for m := range resultC {
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

func main() {
	fn := "measurements.txt"
	if len(os.Args) > 1 {
		fn = os.Args[1]
	}
	r, err := mmap.Open(fn)
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()
	var (
		resultC = make(chan map[string]*Measurements)
		done    = make(chan bool)
		sem     = make(chan bool, runtime.NumCPU())
		wg      sync.WaitGroup
		data    = make(map[string]*Measurements)
	)
	go merger(data, resultC, done)
	var i, j int // start and stop index
	for i < r.Len() {
		j = i + chunkSize
		if j > r.Len() {
			L := j - i
			wg.Add(1)
			sem <- true
			go aggregate(r, i, L, resultC, sem, &wg)
			break
		}
		for {
			if r.At(j) == '\n' {
				break // found newline
			}
			j++
		}
		L := j - i
		wg.Add(1)
		sem <- true
		go aggregate(r, i, L, resultC, sem, &wg)
		i = j + 1
	}
	wg.Wait()
	close(resultC)
	<-done
	keys := maps.Keys(data)
	sort.Strings(keys)
	for _, k := range keys {
		avg := data[k].Sum / float64(data[k].Count)
		fmt.Printf("%s\t%0.2f/%0.2f/%0.2f\n", k, data[k].Min, data[k].Max, avg)
	}
}
