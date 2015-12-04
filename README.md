## GORB [![Build Status](https://travis-ci.org/kobolog/gorb.svg?branch=master)](https://travis-ci.org/kobolog/gorb) [![codecov.io](https://codecov.io/github/kobolog/gorb/coverage.svg?branch=master)](https://codecov.io/github/kobolog/gorb?branch=master)
**Go Routing and Balancing**

This daemon is an IPVS frontend with a REST API interface. You can use it to control local IPVS instance in the Kernel to dynamically register virtual services and backends. It also supports basic TCP and HTTP health checks (called Gorb Pulse).

- **TCP**: tries to establish a TCP connection to the backend's host and port.
- **HTTP**: tries to fetch a specified location from backend's host and port.

Backends which fail to pass the health check will have weights set to zero to inhibit any traffic from being routed into their direction. When a backend comes back online, GORB won't immediately set its weight to the previous value, but instead gradually restore it based on backend's accumulated health statistics.

GORB also supports basic service discovery registration via [Consul](https://www.consul.io): just pass in the Consul endpoint to GORB and it will take care of everything else – your services will be registered with names like `nginx-80-tcp`. Keep in mind that you can use Consul's built-in DNS server to make it even easier to discover your services!

Check out these [slides for my DockerCon EU 2015 talk](http://www.slideshare.net/kobolog/ipvs-for-docker-containers) for more information about IPVS, GORB and how to use it with Docker.

## Configuration

There's not much of a configuration required - only a handlful of options can be specified on the command line:

    gorb [-c <consul-address>] [-f flush-pools] [-i interface] [-l listen-address] | -h

By default, GORB will listen on `:4672`, bind services on `eth0` and keep your IPVS pool intact on launch.

## REST API

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
        "args": {
            "method": "GET",
            "path": "/health",
            "expect": 200
        },
        "interval": "5s"
    },
    "weight": 100
}
```
- `DELETE /service/<service>` removes the specified virtual service and all its backends.
- `DELETE /service/<service>/<backend>` removes the specified backend from the virtual service.
- `GET /service/<service>` returns virtual service configuration.
- `GET /service/<service>/<backend>` returns backend configuration and its health check metrics.

For more information and various configuration options description, consult [`man 8 ipvsadm`](http://linux.die.net/man/8/ipvsadm).

## TODO

- [ ] Add more options for Gorb Pulse: thresholds, exponential back-offs and so on.
- [ ] Support for IPVS statistics (requires GNL2GO support first).
- [ ] Support for FWMARK & DR virtual services (requires GNL2GO support first).
- [x] Add service discovery support, e.g. automatic Consul service registration.
- [ ] Add BGP host-route announces, so that multiple GORBs could expose a service on the same IP across the cluster.
- [ ] Add some primitive UI to present the same action palette but in an user-friendly fashion.
- [ ] Replace command line options with proper configuration via a JSON/YAML/TOML file.
