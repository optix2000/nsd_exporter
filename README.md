# nsd_exporter [![Build Status](https://cloud.drone.io/api/badges/optix2000/nsd_exporter/status.svg)](https://cloud.drone.io/optix2000/nsd_exporter)
Prometheus exporter for NSD (Name Server Daemon)

## Quickstart
`nsd_exporter` will try to autodetect configuration on most Linux distros by reading `/etc/nsd/nsd.conf`.

You will need to launch the process as a user that has permissions to the NSD control certificate and private key (default `/etc/nsd/nsd_control.key`, `/etc/nsd/nsd_control.pem`)

### Examples
```
# Defaults listening to 127.0.0.1:9167/metrics
% nsd_exporter

# Specify a different port to listen on
% nsd_exporter --web.listen-address :9167

# Specify an alternate configuration location to autodetect from
% nsd_exporter --nsd.config /opt/nsd/nsd.conf

# Manually specify NSD socket and certificates
% nsd_exporter --control.ca /etc/nsd/nsd_server.pem --control.key /etc/nsd/nsd_control.key --control.cert /etc/nsd/nsd_control.pem --control.address 127.0.0.1:8952
```

### Add/Modify metrics
If `nsd` has a new version with new metrics or you want to change the description of the existing metrics, you can make changes to the metrics that `nsd_exporter` emits by using your own metrics config file.

1. Download the metrics config from [`config/config.yaml`](https://raw.githubusercontent.com/optix2000/nsd_exporter/master/config/config.yaml)
2. Make any additions or modifications you want.
3. Load it by running `nsd_exporter --metrics-config my-custom-config.yaml`. This will use your config instead of the internal metrics config file.

## Building

```
make
```
