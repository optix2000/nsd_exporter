#!/bin/sh
/nsd_exporter -ca /opt/nsd/certs/nsd -key /opt/nsd/certs/nsd -cert /opt/nsd/certs/nsd -nsd-address 127.0.0.1:8952
