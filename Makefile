build:
	CGO_ENABLED=0 go build -ldflags "-s -w"

generate:

clean:
	rm -f nsd_exporter

all: clean build
