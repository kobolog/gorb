# GORB
**Go Routing and Balancing**

This daemon is actually an IPVS frontend with REST API interface. You can use it to control local IPVS instance in the Kernel to dynamically register virtual services and backends. It also supports basic health checks (called `pulse`):

- TCP: tries to establish a TCP connection to the backend's address and port.
- HTTP: tries to fetch a specified location from backend's address and port.

Backends which fail to pass the health check will have their weights set to zero to inhibit any traffic from being routed their direction. Other than that, it can't do anything fancy yet.

# REST API

- `PUT /service/<service>` creates a new virtual service with provided options:
```json
{
    "address": "10.0.0.1",
    "port": 12345,
    "protocol": "tcp|udp",
    "method": "rr|wrr|lc|wlc|lblc|lblcr|sh|dh|sed|nq",
    "persistent": true
}
```
- `PUT /service/<service>/<backend>` creates a new backend attached to a virtual service:
```json
{
    "address": "10.1.0.1",
    "port": 12346,
    "method": "nat|tunnel",
    "pulse": {
        "type": "tcp|http",
        "interval": "5s",
        "path": "/health (ignored for tcp pulse)"
    },
    "weight": 128
}
```
- `DELETE /service/<service>` removes the specified virtual service and all its backends.
- `DELETE /service/<service>/<backend>` removes the specified backend from the virtual service.
- `GET /service/<service>` returns virtual service configuration.
- `GET /servuce/<service>/<backend>` returns backend configuration and its health check metrics.

For more information and various configuration options description, consult [`man 8 ipvsadm`](http://linux.die.net/man/8/ipvsadm).
