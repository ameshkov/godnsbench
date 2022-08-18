[![Go Report Card](https://goreportcard.com/badge/github.com/ameshkov/godnsbench)](https://goreportcard.com/report/ameshkov/godnsbench)
[![Latest release](https://img.shields.io/github/release/ameshkov/godnsbench/all.svg)](https://github.com/ameshkov/godnsbench/releases)

# godnsbench

A very simple DNS benchmarking tool based on [dnsproxy](https://github.com/AdguardTeam/dnsproxy).

## How to install

* Using homebrew:
    ```
    brew install ameshkov/tap/godnsbench
    ```
* From source:
    ```
    go get github.com/ameshkov/godnsbench
    ```
* You can get a binary from the [releases page](https://github.com/ameshkov/godnsbench/releases).


## Usage

```shell
Usage:
  godnsbench [OPTIONS]

Application Options:
  -a, --address=  Address of the DNS server you're trying to test. Note, that it should include the protocol
                  (tls://, https://, quic://)
  -p, --parallel= The number of connections you would like to open simultaneously (default: 1)
  -q, --query=    The host name you would like to resolve (default: example.org)
  -t, --timeout=  Query timeout in seconds (default: 10)
  -c, --count=    The overall number of queries we should send (default: 10000)
  -v, --verbose   Verbose output (optional)
  -o, --output=   Path to the log file. If not set, write to stdout.

Help Options:
  -h, --help      Show this help message
```
