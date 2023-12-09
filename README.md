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
  -force
        force close connections when server changes (default true)
  -port int
        port number to listen on (default 26257)
  -server value
        a collection of servers to talk to
  -version
        display the current version number
```

### Local example

Dependencies:

* [CockroachDB](https://www.cockroachlabs.com/docs/stable/cockroach-demo)

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
dp \
  --server "localhost:26001" \
  --server "localhost:26002" \
  --port 26000 \
  --http-port 3000 \
  -d
```

Toggle the load balancer to the first cluster

``` sh
curl http://localhost:3000/selected_server \
  -H 'Content-Type:application/json' \
  -d '{"server": "localhost:26001", "force_close": true }'
```

Run a command against the first cluster

``` sh
cockroach sql \
  --url "postgres://root@localhost:26000/defaultdb?sslmode=disable" \
  -e "CREATE TABLE a (id UUID PRIMARY KEY)"
```

Toggle the load balancer to the second cluster

``` sh
curl http://localhost:3000/selected_server \
  -H 'Content-Type:application/json' \
  -d '{"server": "localhost:26002", "force_close": true }'
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

Drain

``` sh
curl -X DELETE http://localhost:3000/selected_server \
  -H 'Content-Type:application/json'
```

### Teardown

``` sh
pkill -9 cockroach dp
rm -rf node1 node2 inflight_trace_dump
```

### Todos

* Wrap terminateSignal and mu in server struct
* Better error handling