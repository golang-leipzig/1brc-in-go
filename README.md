# 1BRC in Go

## Original Task

The One Billion Row Challenge (1BRC) is a fun exploration of how far modern
Java can be pushed for aggregating one billion rows from a text file.  Grab all
your (virtual) threads, reach out to SIMD, optimize your GC, or pull any other
trick, and create the fastest implementation for solving this task!

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

## Baselines

About 10-20s to just iterate sequentually over the file, about 20% cached in
buffers.

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

Compressing data:

```
$ time zstd -c -T0 measurements.txt > measurements.txt.zst

real    0m48.712s
user    3m5.199s
sys     0m17.257s
```

Processing maxed out at about 350MB/s with compression.

```
$ time zstdcat -T0 measurements.txt.zst | pv | cw -l
12.8GiB 0:00:36 [ 358MiB/s] [
 1000000000

real    0m36.750s
user    0m32.334s
sys     0m12.716s
```

With the compressed baseline in 1TB you fould fit about 270B rows and process
them in less than 3 hours (162min).

## Implementations

### Option: "basic"

* just plain defaults, no optimization
* requires linear memory in size of the data
* no parallelism

```
$ time cat ../measurements.txt | go run main.go

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

### Option: "fan-out-fan-in"

Fan-out, fan-in pattern. Read 100K lines, pass to goroutine; e.g. 8 cores, can
work on 8 batches at once. Expecting at most an 8x speedup (e.g. 9min to less
than 2min).

### Option: "scanner"

* save allocs when reading the file, reuse buffer

### Option: "noalloc"

* todo: remove `TrimSpace`, `Split` and friends

### Option: "mmap"

* [why faster?](https://stackoverflow.com/questions/9817233/why-mmap-is-faster-than-sequential-io)
