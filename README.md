# dp
A simple dynamic proxy

### Installation

The easiest way to get started with dp is via Docker:

```sh
docker run -d \
  --name dp \
  --network host \
    codingconcepts/dp:v0.8.0 \
    --port 26257 \
    --ctl-port 3000
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
# Add first server group.
curl http://localhost:3000/groups \
  -H 'Content-Type:application/json' \
  -d '{ "name": "first", "servers": ["localhost:26001"] }'

# Point all traffic to the first sever group.
curl http://localhost:3000/activate \
  -H 'Content-Type:application/json' \
  -d '{ "groups": ["first"], "weights": [100, 0] }'
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
# Add second server group.
curl http://localhost:3000/groups \
  -H 'Content-Type:application/json' \
  -d '{ "name": "second", "servers": ["localhost:26002"] }'

# Point traffic to both server groups equally.
curl http://localhost:3000/activate \
  -H 'Content-Type:application/json' \
  -d '{ "groups": ["first", "second"], "weights": [50, 50] }'

# Point all traffic to the second sever group.
curl http://localhost:3000/activate \
  -H 'Content-Type:application/json' \
  -d '{ "groups": ["first", "second"], "weights": [0, 100] }'
```

Drain and observe everything start to fail

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
