[![Go Report Card](https://goreportcard.com/badge/github.com/ameshkov/godnsbench)](https://goreportcard.com/report/ameshkov/godnsbench)
[![Latest release](https://img.shields.io/github/release/ameshkov/godnsbench/all.svg)](https://github.com/ameshkov/godnsbench/releases)

# godnsbench

A very simple DNS benchmarking tool based on [dnsproxy][dnsproxy].

[dnsproxy]: https://github.com/AdguardTeam/dnsproxy

## How to install

* Using homebrew:
    ```shell
    brew install ameshkov/tap/godnsbench
    ```
* From source:
    ```shell
    go install github.com/ameshkov/godnsbench@latest
    ```
* You can use [a Docker image][dockerimage]:
    ```shell
    docker run --rm ghcr.io/ameshkov/godnsbench --help
    ```
* You can get a binary from the [releases page][releases].

[dockerimage]: https://github.com/ameshkov/godnsbench/pkgs/container/godnsbench

[releases]: https://github.com/ameshkov/godnsbench/releases

## Usage

```shell
Usage:
  godnsbench [OPTIONS]

Application Options:
  -a, --address=    Address of the DNS server you're trying to test. Note, that
                    for encrypted DNS it should include the protocol (tls://,
                    https://, quic://, h3://)
  -p, --parallel=   The number of connections you would like to open
                    simultaneously (default: 1)
  -q, --query=      The host name you would like to resolve. {random} will be
                    replaced with a random string (default: example.org)
  -t, --timeout=    Query timeout in seconds (default: 10)
  -r, --rate-limit= Rate limit (per second) (default: 0)
  -c, --count=      The overall number of queries we should send (default:
                    10000)
      --insecure    Do not validate the server certificate
  -v, --verbose     Verbose output (optional)
  -o, --output=     Path to the log file. If not set, write to stdout.

Help Options:
  -h, --help        Show this help message
```

## Examples

10 connections, 1000 queries to Google DNS using DNS-over-TLS:

```shell
godnsbench -a tls://dns.google -p 10 -c 1000
```

10 connections, 1000 queries to Google DNS using DNS-over-HTTPS with rate limit
not higher than 10 queries per second:

```shell
godnsbench -a https://dns.google/dns-query -p 10 -c 1000 -r 10
```

10 connections, 1000 queries for `example.net` to Google DNS using DNS-over-TLS:

```shell
godnsbench -a https://dns.google/dns-query -p 10 -c 1000 -q example.net
```

10 connections, 1000 queries for `example.net` with timeout 1 second to
AdGuard DNS using DNS-over-QUIC:

```shell
godnsbench -a quic://dns.adguard.com -p 10 -c 1000 -t 1 -q example.net
```

10 connections, 1000 queries for random subdomains of `example.net` with
timeout 1 second to Google DNS using DNS-over-TLS:

```shell
godnsbench -a tls://dns.google -p 10 -c 1000 -t 1 -q {random}.example.net
```
