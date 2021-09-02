#!/bin/bash

docker-compose down
./reset.sh
docker-compose up -d
sleep 2
echo "Will now mine some bitcoins"
node1/bitcoin-cli -named createwallet wallet_name=default load_on_startup=true
node2/bitcoin-cli -named createwallet wallet_name=default load_on_startup=true
node3/bitcoin-cli -named createwallet wallet_name=default load_on_startup=true
while ! ./gen-money.sh ; do echo "Trying to generate bitcoins..." && sleep 0.5; done
echo "Done"
