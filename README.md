# nsd_exporter
Prometheus exporter for NSD (Name Server Daemon)

## Quickstart
`nsd_exporter` will try to autodetect configuration on most Linux distros by reading `/etc/nsd/nsd.conf`.

You will need to launch the process as a user that has permissions to the NSD control certificate and private key (default `/etc/nsd/nsd_control.key`, `/etc/nsd/nsd_control.pem`)

### Examples
```
# Defaults listening to 127.0.0.1:8080/metrics
% nsd_exporter

# Specify a different port to listen on
% nsd_exporter -listen-address :9167

# Specify an alternate configuration location to autodetect from
% nsd_exporter -config-file /opt/nsd/nsd.conf

# Manually specify NSD socket and certificates
% nsd_exporter -ca /etc/nsd/nsd_server.pem -key /etc/nsd/nsd_control.key -cert /etc/nsd/nsd_control.pem -nsd-address 127.0.0.1:8952
```
