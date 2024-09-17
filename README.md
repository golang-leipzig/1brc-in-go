# 1BRC in Go

This started as a

> Lightning Talk at [Leipzig Gophers](https://golangleipzig.space)
> [#42](https://golangleipzig.space/posts/meetup-42-invitation/), [Martin
> Czygan](https://de.linkedin.com/in/martin-czygan-58348842), 2024-04-30 1900,
> Leipzig

but became a playground for various Go solutions to the
[1BRC](https://1brc.dev). Feel free to submit a pull request of discuss
optimizations and ideas in issues, etc.

Note: A treasure trove of optimization approaches in different languages can be
found in [various repos](https://github.com/topics/1brc).

## 1BRC: Original Task

The [One Billion Row Challenge](https://1brc.dev) (1BRC) is a fun exploration
of how far modern Java can be pushed for aggregating one billion rows from a
text file.  Grab all your (virtual) threads, reach out to SIMD, optimize your
GC, or pull any other trick, and create the fastest implementation for solving
this task!

The text file contains temperature values for a range of weather stations.
Each row is one measurement in the format  <string: station name>;<double:
measurement> , with the measurement value having exactly one fractional digit.
The following shows ten rows as an example:

    Hamburg;12.0
    Bulawayo;8.9
    Palembang;38.8
    St. John's;15.2
    Cracow;12.6
    Bridgetown;26.9
    Istanbul;6.2
    Roseau;34.4
    Conakry;31.2
    Istanbul;23.0

The task is to write a Java program which reads the file, calculates the min,
mean, and max temperature value per weather station, and emits the results on
stdout like this (i.e. sorted alphabetically by station name, and the result
values per station in the format  <min>/<mean>/<max> , rounded to one
fractional digit):

> {Abha=-23.0/18.0/59.2, Abidjan=-16.2/26.0/67.3, Abéché=-10.0/29.4/69.0,
> Accra=-10.1/26.4/66.4, Addis Ababa=-
23.7/16.0/67.0, Adelaide=-27.8/17.3/58.5, ...}

## Data Generation

Instructions for generating a dataset are in the original repo: [https://github.com/gunnarmorling/1brc](https://github.com/gunnarmorling/1brc)

Here is a glimpse:

```
$ head measurements.txt
Tamale;27.5
Bergen;9.6
Lodwar;37.1
Whitehorse;-3.8
Ouarzazate;19.1
Erbil;20.2
Naha;6.5
Milan;2.1
Riga;-0.7
Bilbao;17.9

$ stat --printf "%F %N %s\n" measurements.txt
regular file 'measurements.txt' 13795406386
```

About 13GB.

## TL;DR

Some current timings:

* i7-7500U: 65s (~14x slower, 224.00 GFLOP/s)
* i7-8550U: 37s (~8x slower, 512.00 GFLOP/s)
* i9-13900T: 4.5s (3.35 TFLOP/s) (**reference**)
* Intel Xeon Gold 6326: 4.3s (~1.04x faster, 3.56TFLOP/s)
* AMD E-350 (2011): 316s (~261x slower, 12.8 GFLOP/s)

![](static/657582.gif)

With a bit of cheating, using a custom-made collision free map, the reference
machine takes 2.2s.

## Baselines

About 10-20s to just iterate sequentually over the file, about 20% cached in
buffers. Using [pcstat](https://github.com/tobert/pcstat), and [cw](https://github.com/Freaky/cw).

```
$ pcstat measurements.txt
+------------------+----------------+------------+-----------+---------+
| Name             | Size (bytes)   | Pages      | Cached    | Percent |
|------------------+----------------+------------+-----------+---------|
| measurements.txt | 13795406386    | 3368020    | 657316    |  19.516 |
+------------------+----------------+------------+-----------+---------+

$ time wc -l measurements.txt
1000000000 measurements.txt

real    0m18.503s
user    0m10.849s
sys     0m7.412s

$ LC_ALL=C time wc -l measurements.txt
1000000000 measurements.txt
11.21user 7.71system 0:19.12elapsed 98%CPU (0avgtext+0avgdata 1536maxresident)k
26943912inputs+0outputs (0major+134minor)pagefaults 0swaps

$ time cat measurements.txt | pv > /dev/null
12.8GiB 0:00:15 [ 845MiB/s]

real    0m15.566s
user    0m0.680s
sys     0m14.461s

$ time cw -l measurements.txt # rust "wc"
1000000000 measurements.txt

real    0m9.961s
user    0m1.449s
sys     0m7.901s
```

On a [i9-13900T](https://www.intel.com/content/www/us/en/products/sku/230498/intel-core-i913900t-processor-36m-cache-up-to-5-30-ghz/specifications.html) a plain `wc -l` takes 5.3s, and `cw -l` 2.8.

Compressing data:

```
$ time zstd -c -T0 measurements.txt > measurements.txt.zst

real    0m48.712s
user    3m5.199s
sys     0m17.257s
```

Reading compressed data; data point (about 400M/s).

```
$ zstdcat -T0 measurements.txt.zst | pv > /dev/null
12.8GiB 0:00:32 [ 399MiB/s] [
```

Processing maxes out at about 350MB/s with compression.

```
$ time zstdcat -T0 measurements.txt.zst | pv | cw -l
12.8GiB 0:00:36 [ 358MiB/s] [
1000000000

real    0m36.750s
user    0m32.334s
sys     0m12.716s
```

The above was all SATA SSD. With nvme drives (raid0) we can read the whole file
sequentially at 4.5G/s (at 100% cached).

```
$ time cat /var/data/tmp/measurements.txt | pv > /dev/null
12.8GiB 0:00:02 [4.51GiB/s] [

real    0m2.851s
user    0m0.059s
sys     0m3.204s
```

With cold cache, still 5s.

```
$ time cat /var/data/tmp/measurements.txt | pv > /dev/null
12.8GiB 0:00:05 [2.49GiB/s] [                        <=>                                                                                                                                                                                      ]

real    0m5.166s
user    0m0.078s
sys     0m5.419s
```

With the compressed baseline 1TB of data would fit about 270B rows and process
them in about 3 hours (162min).

## Implementations

### Option: "basic"

* [x] just plain defaults, no optimization
* [x] requires linear memory in size of the data
* [x] no parallelism

```
$ time cat measurements.txt | ./1brc-basic

real    4m17.288s
user    4m21.518s
sys     0m27.459s
```

According to the official site, this somewhat matches the reference
implementation (with 4m13s:
[leaderboard](https://1brc.dev/#global-leaderboard)).

### Option: "savemem"

Compute metrics on the fly, constant memory usage. Since we do 1B comparisons
(instead of just appends or writes to memory), we are actually slower (even
though we have fewer allocations); around 9min.

* [x] struct, update after each line

```
$ time cat measurements.txt | ./1brc-savemem
real    9m31.114s
user    9m40.704s
sys     1m26.789s
```

Thesis: Instead of a memory write (as in the baseline) we have to do a read,
compare and potential write operation. That may be roughly twice the work.

### Option: "fan-out, fan-in"

Fan-out, fan-in pattern. Read 100K lines, pass to goroutine; e.g. 8 cores, can
work on 8 batches at once. Expecting at most an 8x speedup (e.g. 9min to less
than 2min). Well, with batch size 20M we are down to 3:25 on an 8-core machine; we do not max out the cores.

* [x] fan-out fan-in
* [x] bufio.Reader.ReadString

```
$ cat measurements.txt | ./1brc-fanout

real    3m25.233s
user    12m53.138s
sys     1m10.568s
```

On a 32-core i9-13900T we are down to 1:05:

```
$ time zstdcat -T0 measurements.txt.zst | pv | ./1brc-fanout
...
Ürümqi  -41.70/56.40/7.41
İzmir   -33.10/73.30/17.91

real    1m5.707s
user    4m22.780s
sys     0m14.308s
```

### Option: "scanner"

* [x] scanner.Text, save allocs when reading the file, reuse buffer
* [x] fan-out fan-in

```
$ cat measurements.txt | ./1brc-scan
real    2m42.830s
user    10m56.898s
sys     1m0.452s
```

On a 32-core CPU we reach 1m8.088s with this approach (independent of compression, SATA, nvme).

### Option: "noalloc"

* [x] scanner.Text, save allocs when reading the file, reuse buffer
* [x] fan-out fan-in
* [x] fewer allocations

```
$ cat measurements.txt | ./1brc-scan-noalloc
real    2m14.987s
user    7m6.461s
sys     0m47.624s
```

On a 32-core CPU we reach 1m1.968s.

### Option: "mmap"

* [why faster?](https://stackoverflow.com/questions/9817233/why-mmap-is-faster-than-sequential-io)
* [How does mmap improve file reading speed?](https://stackoverflow.com/questions/37172740/how-does-mmap-improve-file-reading-speed)
* [read line by line in the most efficient way *platform specific*](https://stackoverflow.com/questions/33616284/read-line-by-line-in-the-most-efficient-way-platform-specific/33620968#33620968)

A pure, read-from-mmap iteration in 128MB chunks takes 11s. With 64MB chunks we are down to 5s.

```
$ cat measurements.txt | ./1brc-mmap
real    0m45.136s
user    5m31.853s
sys     0m6.745s
```

On a 32-core machine we are down to real 0m6.684s.

### Option: "use int, not float"

You can manually parse a temp like '-16.7' into -167 and then back convert to float64 at output time.

```
$ cat measurements.txt | ./1brc-mmap-int
real    0m41.618s
user    4m53.000s
sys     0m11.347s
```

### Option: "faster float parse"

* [fastfloat/parse.go#L203](https://github.com/valyala/fastjson/blob/6dae91c8e11a7fa6a257a550b75cba53ab81693e/fastfloat/parse.go#L203)

```
$ cat measurements.txt | ./1brc-mmap-float

real    0m38.261s
user    4m22.127s
sys     0m10.156s
```

On a 32-core machine, we are down to 0m5.880s.

On the reference CX33 machine:

```
$ cat measurements.txt | ./1brc-mmap-float

real    0m17.723s
user    2m10.490s
sys     0m4.588s
```

### Option: "static map"

A pprof run showed, that map access was the most expensive part of the process.
This is cheating, but to test the potential, we used a custom, collision-free
map. This halved the processing time (e.g. from 4.5s to 2.2s) - and still,
"calculateIndex" would remain the most expensive part.

```go
// calculateIndex, interestingly the most expensive part of the program.
func calculateIndex(s string) (index int) {
	for i, c := range s {
		index = index + i*(37+int(c))
	}
	return index % 16384
}
```

Run (i9-13900T):

```
$ cat measurements.txt | ./1brc-mmap-int-static-map

real    0m2.416s
user    1m3.471s
sys     0m1.439s
```

## Preliminary Summary

"faster float" is about 3x slower then the fastest JVM implementation. The code contains a
nice comment section, with about 30 different ideas (and their impact):

* [CalculateAverage_royvanrijn.java#L36-L67](https://github.com/gunnarmorling/1brc/blob/db064194be375edc02d6dbcd21268ad40f7e2869/src/main/java/dev/morling/onebrc/CalculateAverage_royvanrijn.java#L36-L67)

## TODO

* [ ] profile guided optimization

Using [gperf](https://www.gnu.org/software/gperf/) to find a perfect hash
function, generate C, transpile to Go, with [ccgo](https://pkg.go.dev/modernc.org/ccgo/v4).

```
$ gperf cities.txt > perfect.c
$ ccgo perfect.c # add #include <stddef.h>
$ cat perfect.go
```
