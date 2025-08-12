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
curl -s http://localhost:3000/ports/26000/groups \
--json '{ "name": "primary", "servers": ["localhost:26001"] }'

curl -s http://localhost:3000/ports/26000/groups \
--json '{ "name": "standby", "servers": ["localhost:26002"] }'

curl -s http://localhost:3000/ports/8000/groups \
--json '{ "name": "primary", "servers": ["localhost:8001"] }'

curl -s http://localhost:3000/ports/8000/groups \
--json '{ "name": "standby", "servers": ["localhost:8002"] }'

# Point all traffic to primary sever groups.
curl -s http://localhost:3000/ports/26000/activate \
--json '{ "groups": ["primary"], "weights": [100] }'

curl -s http://localhost:3000/ports/8000/activate \
--json '{ "groups": ["primary"], "weights": [100] }'

# View load balancer configuration.
curl -s http://localhost:3000/ports | jq
```

At this point, the configuration will resemble the following, with two top-level ports, each with a primary and a secondary and a single server for each (although this can be configured to support an arbitrary number of servers):

```json
{
  "26000": {
    "primary": {
      "weight": 100,
      "servers": [
        "localhost:26001"
      ]
    },
    "standby": {
      "weight": 0,
      "servers": [
        "localhost:26002"
      ]
    }
  },
  "8000": {
    "primary": {
      "weight": 100,
      "servers": [
        "localhost:8001"
      ]
    },
    "standby": {
      "weight": 0,
      "servers": [
        "localhost:8002"
      ]
    }
  }
}
```

Run a command against the first cluster (via the load balancer):

``` sh
cockroach sql \
--url "postgres://root@localhost:26000/defaultdb?sslmode=disable" \
-e "SELECT now(), crdb_internal.cluster_id()"
```

Point traffic to the standby cluster.

``` sh
curl -s http://localhost:3000/ports/26000/activate \
--json '{ "groups": ["standby"], "weights": [100] }'

curl -s http://localhost:3000/ports/8000/activate \
--json '{ "groups": ["standby"], "weights": [100] }'
```

Run the same command again but this time against the standby cluster (via the load balancer):

``` sh
cockroach sql \
--url "postgres://root@localhost:26000/defaultdb?sslmode=disable" \
-e "SELECT now(), crdb_internal.cluster_id()"
```

Drain the load balancer

``` sh
curl -s http://localhost:3000/ports/26000/activate \
--json '{"groups": []}'

curl -s http://localhost:3000/ports/8000/activate \
--json '{"groups": []}'
```

Run the same command again but this time it will fail because the load balancer is drained:

``` sh
cockroach sql \
--url "postgres://root@localhost:26000/defaultdb?sslmode=disable" \
-e "SELECT now(), crdb_internal.cluster_id()"
```

### Teardown

``` sh
pkill -9 cockroach dp main
rm -rf inflight_trace_dump
```

### Todos

* Wrap terminateSignal and mu in server struct
* Better error handling
