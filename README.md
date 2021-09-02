# bitcoin-processing

bitcoin-processing is a payment gateway for Bitcoin. It provides an API for
sending an receiving Bitcoin payments, essentially providing a higher-level API
around a wallet provided by Bitcoin Core node.

This code provides [two binaries](/cmd) - `bitcoin-processing`, which is a main
processing daemon that connects to Bitcoin node and is controlled by HTTP API,
and `bitcoin-processing-client`, which is a command-line client for the daemon.

## Building

Both binaries should be built using regular `go build`.
```bash
(cd cmd/bitcoin-processing && go build)
(cd cmd/bitcoin-processing-client && go build)
```

## Usage

`bitcoin-processing` requires a **running Bitcoin Core node** and a **PostgreSQL
database**. It also requires a config file.

### Bitcoin node

A full Bitcoin Core node is required. It's config may look like this:

```ini
testnet=0
regtest=0
printtoconsole=1
rpcuser=bitcoinrpc
rpcpassword=SET_THIS_TO_SECURE_PASSWORD
# rpcallowip can be, for example, set to 127.0.0.1/32 if bitcoin-processing will
# be running on the same host. If bitcoin node is run in Docker, it is often set
# to 0.0.0.0/0
rpcallowip=SET_THIS_TO_ALLOWED_NETMASK
# rpcbind can be, for example, set to 127.0.0.1:8332 if bitcoin-processing will
# be running on the same host. If bitcoin node is run in Docker, it is often set
# to 127.0.0.1:8332
rpcbind=SET_THIS_TO_BIND_ADDRESS
```

After Bitcoin node has started and initialized, a wallet should be created in
it. This can be done with this request:
```bash
curl -vv -XPOST --user bitcoinrpc http://127.0.0.1:8332 \
    --data '{"jsonrpc": "1.0","method":"createwallet", "params":{"wallet_name":"default","load_on_startup":true}}'
```

### PostgreSQL database

PostgreSQL >= 10 is supported. In it, user and DB `bitcoin_processing` should be
created.

```sql
CREATE USER bitcoin_processing WITH PASSWORD 'POSTGRES_PASSWORD_ALSO_SET_TO_SECURE';

CREATE DATABASE bitcoin_processing OWNER bitcoin_processing;
```

In the commands above, password can be omitted if processing will run as
`bitcoin_processing` OS user and connect to Postgres using UNIX socket (in that
case peer authentication can be used).

After user and DB are created, DB schema should be initialized with
```bash
psql -Ubitcoin_processing --host POSTGRES_HOST < tools/init-db.sql
```

### Config

`bitcoin-processing` is configured using a YAML file, config looks like this

```yaml
transaction:
  callback:
    url: http://192.168.37.11:8080/wallets/cb
  max_confirmations: 1
api:
  http:
    address: 192.168.37.2:8000
storage:
  type: postgres
  dsn: >
    host=127.0.0.1 dbname=bitcoin_processing
    user=bitcoin_processing password=POSTGRES_PASSWORD_ALSO_SET_TO_SECURE
    sslmode=disable
bitcoin:
  node:
    address: 127.0.0.1:8332
    user: bitcoinrpc
    password: SET_THIS_TO_SECURE_PASSWORD

wallet:
   min_withdraw_without_manual_confirmation: 0.1
```

Here, `storage` describes connection info for Postgres DB, `bitcoin.node`
contains info necessary for connecting to Bitcoin node RPC API.
`transaction.callback` is a URL of HTTP callback, to which `bitcoin-processing`
will send notification requests. `api.http.address` contains an address HTTP
API server will listen on.

### Running

After Postgres and Bitcoin node are ready and config is written, processing can
be started with
```bash
./bitcoin-processing -c ./config.yml
```
It will auto-generate hot wallet address it one was not generated yet, and will
start listening on API server address.

## Running tests

**Unit tests** are run with
```bash
go test ./...
```
Also, there are **integration tests**. They require binaries to be built first,
also, they require Docker - they will start several Docker containers.
They can be run with
```bash
go test -v -timeout=20m -tags integration ./integrationtests
```
