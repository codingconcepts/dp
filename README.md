# lb
A simple TCP load balancer

### Run locally

Start insecure cockroach cluster

``` sh
cockroach demo --sql-port 26001 --http-port 8001 --insecure
```

Start secure cockroach cluster

``` sh
cockroach demo --sql-port 26002 --http-port 8002
```

Create user for secure cluster

``` sql
CREATE USER rob WITH PASSWORD 'password';
GRANT ALL PRIVILEGES ON DATABASE defaultdb TO rob;
```

Run the load balancer

``` sh
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
  --url "postgres://rob:password@localhost:26002/defaultdb?sslmode=allow"
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
rm -rf node1 node2 inflight_trace_dump
```

### Todos

* Better error handling