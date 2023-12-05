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

Toggle the load balancer to the first cluster

```
[0] drain
[1] localhost:26001
[2] localhost:26002

Selected: localhost:26001

> 1
```

Run a command against the first cluster

``` sh
cockroach sql \
  --url "postgres://root@localhost:26000/defaultdb?sslmode=disable" \
  -e "CREATE TABLE a (id UUID PRIMARY KEY)"
```

Toggle the load balancer to the second cluster

```
[0] drain
[1] localhost:26001
[2] localhost:26002

Selected: localhost:26002

> 2
```

Run a command against the other cluster

``` sh
cockroach sql \
  --url "postgres://rob:password@localhost:26000/defaultdb?sslmode=allow" \
  -e "CREATE TABLE b (id UUID PRIMARY KEY)"
```

Confirm that the tables have been created in the expected clusters

``` sh
cockroach sql \
  --url "postgres://root@localhost:26001/defaultdb?sslmode=disable" \
  -e "SHOW TABLES"

cockroach sql \
  --url "postgres://rob:password@localhost:26002/defaultdb?sslmode=allow" \
  -e "SHOW TABLES"
```

### Teardown

``` sh
pkill -9 cockroach lb
rm -rf node1 node2 inflight_trace_dump
```

### Todos

* Better error handling