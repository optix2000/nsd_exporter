build: generate
	go build
generate:
	go generate
clean:
	rm -f nsd_exporter
build-docker:
	docker build -t nsd_exporter . 
all: clean build
