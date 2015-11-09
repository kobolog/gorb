# GORB ![Travis-CI](https://travis-ci.org/kobolog/gorb.svg?branch=master)
**Go Routing and Balancing**

This daemon is actually an IPVS frontend with REST API interface. You can use it to control local IPVS instance in the Kernel to dynamically register virtual services and backends. It also supports basic health checks (called Gorb Pulse):

- **TCP**: tries to establish a TCP connection to the backend's address and port.
- **HTTP**: tries to fetch a specified location from backend's address and port.

Backends which fail to pass the health check will have their weights set to zero to inhibit any traffic from being routed their direction. Other than that, it can't do anything fancy yet.

# Configuration

There's not much of a configuration required. The only few options can be specified on the command line:

    gorb [-l <listen-address>] [-d <device>] [-f flush-pools] | -h

By default, GORB will listen on `:4672`, bind services on `eth0` and keep your IPVS pool intact on launch.

# REST API

- `PUT /service/<service>` creates a new virtual service with provided options. If `host` is omitted, GORB will pick an
address automatically based on the configured default device:
```json
{
    "host": "10.0.0.1",
    "port": 12345,
    "protocol": "tcp|udp",
    "method": "rr|wrr|lc|wlc|lblc|lblcr|sh|dh|sed|nq|...",
    "persistent": true
}
```
- `PUT /service/<service>/<backend>` creates a new backend attached to a virtual service:
```json
{
    "host": "10.1.0.1",
    "port": 12346,
    "method": "nat|tunnel",
    "pulse": {
        "type": "none|tcp|http",
        "interval": "5s",
        "path": "/health (ignored for tcp pulse)"
    },
    "weight": 128
}
```
- `DELETE /service/<service>` removes the specified virtual service and all its backends.
- `DELETE /service/<service>/<backend>` removes the specified backend from the virtual service.
- `GET /service/<service>` returns virtual service configuration.
- `GET /service/<service>/<backend>` returns backend configuration and its health check metrics.

For more information and various configuration options description, consult [`man 8 ipvsadm`](http://linux.die.net/man/8/ipvsadm).

# TODO

- Add more options for Gorb Pulse: thresholds, HTTP verbs and expected responses, exponential back-offs and so on.
- Support for IPVS statistics (requires GNL2GO support first).
- Support for FWMARK & DR virtual services.
- Add BGP host-route announces, so that multiple GORBs could expose a service on the same IP across the cluster.
- Add some primitive UI to present the same action palette but in an user-friendly fashion.
- Optional: add dynamic DNS support, so that GORBs could register new service names on some specified subdomain.
