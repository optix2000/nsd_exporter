FROM alpine:latest
RUN apk add openssl nsd unbound
RUN nsd-control-setup
RUN unbound-control-setup
