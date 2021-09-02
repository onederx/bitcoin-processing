#!/bin/bash -e

node2/bitcoin-cli generatetoaddress 3 `node2/bitcoin-cli getnewaddress | tr '\r' ' '`
sleep 1
node3/bitcoin-cli generatetoaddress 110 `node3/bitcoin-cli getnewaddress | tr '\r' ' '`
