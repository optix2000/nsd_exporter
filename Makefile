build: generate
	go build
generate:
	go generate
clean:
	rm -f nsd_exporter
all: clean build
