#!/bin/bash

docker-compose down
./reset.sh
docker-compose up -d
sleep 2
echo "Will now mine some bitcoins"
while ! ./gen-money.sh ; do echo "Trying to generate bitcoins..." && sleep 0.5; done
echo "Done"
