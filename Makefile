SHELL := /bin/bash
TARGETS := \
	1brc-basic \
	1brc-savemem \
	1brc-fanout \
	1brc-scan

.PHONY: all
all: $(TARGETS)


%: cmd/%/main.go
	go build -o $@ $^

.PHONY: clean
clean:
	rm -rf $(TARGETS)

