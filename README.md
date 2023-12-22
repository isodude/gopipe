# GoPipe

Small go-program meant to act like socat, but not as fancy, but with netns support built in.

## Alternatives

### socat

It's possible to solve everything with socat, but it's quite much and I wanted something very simple. And I liked the exercise to work with network namespaces in Go.

### systemd-socket-proxyd

Does not support TLS on sockets sadly.

## Why

Pipe between two network namespaces either locally or remote servers. Only support TCP at the moment.

Does support systemd.socket which means you can use it as the below.

gopipe act as a bridge for outbound service (which may lack TLS all together) to inbound.service.

gopipe.socket
```
[Socket]
ListenStream=<inbound-ip>:443
```

gopipe.service
```
[Unit]
JoinsNamespaceOf=inbound.service
Requires=inbound.service gopipe.socket
After=inbound.service gopipe.socket

[Service]
Sockets=gopipe.socket
DynamicUser=yes
ExecStart=gopipe --listen.addr=FD:3 --listen.tls.cert-file=default.crt --listen.tls.key-file=default.key --connect 127.0.0.1:80
PrivateNetwork=yes
```

gopipe@outbound.service
```
[Unit]
JoinsNamespaceOf=%i.service
Requires=%i.service
After=%i.service

[Service]
ExecStart=gopipe --listen.netns.systemd-unit=outbound.service --listen.addr=127.0.0.1:80 --client.tls.cert-file=default.crt --client.tls.key-file=default.key --connect <inbound-ip>:443
```

`gopipe --help`

```
Usage:
  gopipe [OPTIONS]

Application Options:
      --debug

client:
      --listen.debug
      --listen.addr=                 listen on address
      --listen.user=                 change to user on listen thread
      --listen.group=                change to group on listen thread
      --listen.uid=                  change user on listen thread
      --listen.gid=                  change group on listen thread

tls:
      --listen.tls.ca-file=          TLS CA file
      --listen.tls.cert-file=        TLS Cert file
      --listen.tls.key-file=         TLS Key file
      --listen.tls.debug
      --listen.tls.allowed-dns-name= Allowed DNS names

netns:
      --listen.netns.docker-name=    A docker identifier
      --listen.netns.net-name=       A iproute2 netns name
      --listen.netns.path=           A netns path
      --listen.netns.systemd-unit=   A systemd unit name
      --listen.netns.pid=            Process ID of a running process
      --listen.netns.tid=            Thread ID of a running thread inside a process
      --listen.netns.debug

client:
      --client.debug
      --client.addr=                 connect to address
      --client.source-ip=            IP used as source address

tls:
      --client.tls.ca-file=          TLS CA file
      --client.tls.cert-file=        TLS Cert file
      --client.tls.key-file=         TLS Key file
      --client.tls.debug

netns:
      --client.netns.docker-name=    A docker identifier
      --client.netns.net-name=       A iproute2 netns name
      --client.netns.path=           A netns path
      --client.netns.systemd-unit=   A systemd unit name
      --client.netns.pid=            Process ID of a running process
      --client.netns.tid=            Thread ID of a running thread inside a process
      --client.netns.debug

Help Options:
  -h, --help                         Show this help message

```
