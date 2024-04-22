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
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"

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

func main() {
	var data = make(map[string]*Measurements)
	br := bufio.NewReader(os.Stdin)
	for {
		line, err := br.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		parts := strings.Split(line, ";")
		if len(parts) != 2 {
			log.Fatalf("expected two fields: %s", line)
		}
		name := strings.TrimSpace(parts[0])
		temp, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil {
			log.Fatalf("invalid temp: %f", parts[1])
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
	keys := maps.Keys(data)
	sort.Strings(keys)
	for _, k := range keys {
		avg := data[k].Sum / float64(data[k].Count)
		fmt.Printf("%s\t%0.2f/%0.2f/%0.2f\n", k, data[k].Min, data[k].Max, avg)
	}
}
