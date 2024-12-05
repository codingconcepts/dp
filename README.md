# dp
A simple dynamic proxy

### Installation

Find the release that matches your architecture on the [releases](https://github.com/codingconcepts/dp/releases) page.

Download the tar, extract the executable, and move it into your PATH:

```
$ tar -xvf dp_0.1.0_macos_arm64.tar.gz
```

### Usage

```
$ dp -h

Usage of dp:
  -ctl-port int
        port number for proxy control requests (default 3000)
  -debug
        enable debug-level logging
  -port int
        port number for proxy requests (default 26257)
  -version
        show the application version
```

### Local example

Dependencies:

* [CockroachDB](https://www.cockroachlabs.com/docs/stable/cockroach-demo)

Start first cluster

``` sh
cockroach demo --sql-port 26001 --http-port 8001 --insecure
```

Start second cluster

``` sh
cockroach demo --sql-port 26002 --http-port 8002 --insecure
```

Run the load balancer

``` sh
go run dp.go \
--port 26000 \
--ctl-port 3000
```

Add and activate the first cluster in the load balancer

``` sh
curl http://localhost:3000/groups \
  -H 'Content-Type:application/json' \
  -d '{"name": "first", "servers": ["localhost:26001"]}'

curl http://localhost:3000/activate \
  -H 'Content-Type:application/json' \
  -d '{"groups": ["first"]}'
```

Run a command against the first cluster (making use of the [see](https://github.com/codingconcepts/see) CLI)

``` sh
see -n 1 \
cockroach sql \
--url "postgres://root@localhost:26000/defaultdb?sslmode=disable" \
-e "SELECT crdb_internal.cluster_id()"
```

Add the second cluster to the load balancer

``` sh
curl http://localhost:3000/groups \
  -H 'Content-Type:application/json' \
  -d '{"name": "second", "servers": ["localhost:26002"]}'
```

Toggle the load balancer to the second cluster and observe the cluster id change

``` sh
curl http://localhost:3000/activate \
  -H 'Content-Type:application/json' \
  -d '{"groups": ["second"]}'
```

Drain and observe everything go to shit

``` sh
curl http://localhost:3000/activate \
  -H 'Content-Type:application/json' \
  -d '{"groups": []}'
```

### Teardown

``` sh
pkill -9 cockroach dp main
rm -rf inflight_trace_dump
```

### Todos

* Wrap terminateSignal and mu in server struct
* Better error handling
