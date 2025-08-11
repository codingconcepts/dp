# dp
A simple dynamic proxy

### Installation

The easiest way to get started with dp is via Docker:

```sh
docker run -d \
  --name dp \
  --network host \
    codingconcepts/dp:v0.9.0 \
    --port 26257 \
    --port 8080 \
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

Run load balancers

``` sh
go run dp.go --port 26000 --port 8000 --ctl-port 3000
```

Add and activate the first cluster in the load balancer

``` sh
# Add server groups.
curl http://localhost:3000/ports/26000/groups \
--json '{ "name": "primary", "servers": ["localhost:26001"] }'

curl http://localhost:3000/ports/26000/groups \
--json '{ "name": "standby", "servers": ["localhost:26002"] }'

curl http://localhost:3000/ports/8000/groups \
--json '{ "name": "primary", "servers": ["localhost:8001"] }'

curl http://localhost:3000/ports/8000/groups \
--json '{ "name": "standby", "servers": ["localhost:8002"] }'

# Point all traffic to primary sever groups.
curl http://localhost:3000/ports/26000/activate \
--json '{ "groups": ["primary"], "weights": [100] }'

curl http://localhost:3000/ports/8000/activate \
--json '{ "groups": ["primary"], "weights": [100] }'
```

Run a command against the first cluster (making use of the [see](https://github.com/codingconcepts/see) CLI)

``` sh
see -n 1 \
cockroach sql \
--url "postgres://root@localhost:26000/defaultdb?sslmode=disable" \
-e "SELECT now(), crdb_internal.cluster_id()"
```

Point traffic to the standby cluster.

``` sh
curl http://localhost:3000/ports/26000/activate \
--json '{ "groups": ["standby"], "weights": [100] }'

curl http://localhost:3000/ports/8000/activate \
--json '{ "groups": ["standby"], "weights": [100] }'
```

Drain and observe everything start to fail

``` sh
curl http://localhost:3000/ports/26000/activate \
--json '{"groups": []}'

curl http://localhost:3000/ports/8000/activate \
--json '{"groups": []}'
```

### Teardown

``` sh
pkill -9 cockroach dp main
rm -rf inflight_trace_dump
```

### Todos

* Wrap terminateSignal and mu in server struct
* Better error handling
