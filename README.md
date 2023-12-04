# lb
A simple TCP load balancer

### Run locally

Start clusters and lb

``` sh
cockroach start-single-node \
  --insecure \
  --listen-addr=localhost:26001 \
  --http-addr=localhost:8001 \
  --store node1 \
  --background

cockroach start-single-node \
  --insecure \
  --listen-addr=localhost:26002 \
  --http-addr=localhost:8002 \
  --store node2 \
  --background

go run lb.go \
  -server "localhost:26001" \
  -server "localhost:26002" \
  -port 26000
```

Run a command against one cluster

``` sh
cockroach sql \
  --host localhost:26000 \
  --insecure \
  -e "CREATE TABLE a (id UUID PRIMARY KEY)"
```

Change lb to point to the other server

Run a command against the other cluster

``` sh
cockroach sql \
  --host localhost:26000 \
  --insecure \
  -e "CREATE TABLE b (id UUID PRIMARY KEY)"
```

Confirm that the tables have been created in the expected clusters

``` sh
cockroach sql \
  --host localhost:26001 \
  --insecure \
  -e "SHOW TABLES"

cockroach sql \
  --host localhost:26002 \
  --insecure \
  -e "SHOW TABLES"
```

### Teardown

``` sh
pkill -9 cockroach
rm -rf node1 node2
```

### Todos

* Allow server to block requests (for things like cutovers)
* Better error handling