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
	"slices"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/exp/maps"
)

var data = make(map[string][]float32)

func main() {
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
		data[name] = append(data[name], float32(temp))
	}
	keys := maps.Keys(data)
	sort.Strings(keys)
	for _, k := range keys {
		min := slices.Min(data[k])
		max := slices.Max(data[k])
		var sum float32 = 0.0
		for _, t := range data[k] {
			sum = sum + t
		}
		avg := sum / float32(len(data[k]))
		fmt.Printf("%s\t%0.2f/%0.2f/%0.2f\n", k, min, max, avg)
	}

}
