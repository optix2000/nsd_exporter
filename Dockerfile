FROM golang:alpine AS builder
WORKDIR /opt
COPY . /opt
# Using go get.
RUN go get -d -v
RUN go build -o /nsd_exporter

FROM alpine
WORKDIR /
COPY --from=builder  /nsd_exporter /nsd_exporter
ADD certs/nsd /opt/nsd/certs/nsd
ADD entry-point.sh /entry-point.sh
RUN chmod +x /nsd_exporter /entry-point.sh
ENTRYPOINT ["/entry-point.sh"]
