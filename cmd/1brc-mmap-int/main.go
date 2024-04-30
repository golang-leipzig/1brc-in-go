// instead of parsing float, parse an int instead
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"sort"
	"sync"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/mmap"
)

var cpuprofile = flag.String("cpuprofile", "", "file to write cpu profile to")

const chunkSize = 67108864 // 33554432 // 67108864

// Measurements, as there is no need to keep all numbers around, we can compute
// them on the fly.
type Measurements struct {
	Min   int
	Max   int
	Sum   int
	Count int
}

func (m *Measurements) Add(v int) {
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

// parseTempToInt turns '-16.7' into -167. It is up to the caller to take care
// of the back conversion.
func parseTempToInt(p []byte) int {
	var result int
	var pos = 1
	for i := len(p) - 1; i > -1; i-- {
		if p[i] == '.' {
			continue
		} else if p[i] == '-' {
			return -result
		} else {
			var digit = p[i] - '0'
			result = result + int(digit)*pos
			pos = 10 * pos
		}
	}
	return result
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
			temp := parseTempToInt(buf[k+1 : l])
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
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	fn := "measurements.txt"
	if flag.NArg() > 0 {
		fn = flag.Arg(0)
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
		avg := (float64(data[k].Sum) / 10) / float64(data[k].Count)
		fmt.Printf("%s\t%0.2f/%0.2f/%0.2f\n", k, float64(data[k].Min)/10, float64(data[k].Max)/10, avg)
	}
}
