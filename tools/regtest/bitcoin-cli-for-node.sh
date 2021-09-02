#!/bin/bash

node=`basename "$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null && pwd )"`

docker-compose exec $node bitcoin-cli -rpcwait -regtest $@