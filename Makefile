MAKEFLAGS := --jobs=$(shell nproc)
SHELL := /bin/bash
TARGETS := \
	1brc-basic \
	1brc-savemem \
	1brc-fanout \
	1brc-scan \
	1brc-scan-noalloc \
	1brc-mmap \
	1brc-mmap-float \
	1brc-mmap-int \
	1brc-mmap-int-tweaks \
	1brc-mmap-int-static-map

.PHONY: all
all: $(TARGETS)


%: cmd/%/main.go
	go build -o $@ $^

.PHONY: clean
clean:
	rm -rf $(TARGETS)

