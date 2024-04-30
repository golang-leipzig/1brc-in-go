SHELL := /bin/bash
TARGETS := \
	1brc-basic \
	1brc-savemem \
	1brc-fanout \
	1brc-scan \
	1brc-scan-noalloc \
	1brc-mmap \

.PHONY: all
all: $(TARGETS)


%: cmd/%/main.go
	go build -o $@ $^

.PHONY: clean
clean:
	rm -rf $(TARGETS)
