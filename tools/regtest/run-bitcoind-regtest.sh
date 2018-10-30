#!/bin/bash

docker run -d --rm -v /home/asterite/wonderx/regtest/data:/bitcoin --name=bitcoind-regtest-node \
    -p 127.0.0.1:14881:14881 \
    -p 127.0.0.1:14882:14882 \
    -p 14883:14883 -p 14884:14884 -p 127.0.0.1:18443:18443  -p 127.0.0.1:18444:18444 \
    kylemanna/bitcoind -regtest \
    -rpcallowip=0.0.0.0/0 \
    -zmqpubhashtx=tcp://0.0.0.0:14881 -zmqpubhashblock=tcp://0.0.0.0:14882 -zmqpubrawblock=tcp://0.0.0.0:14883 -zmqpubrawtx=tcp://0.0.0.0:14884



    kylemanna/bitcoind -regtest \
    -rpcallowip=0.0.0.0/0 \
    -zmqpubhashtx=tcp://0.0.0.0:14881 -zmqpubhashblock=tcp://0.0.0.0:14882 -zmqpubrawblock=tcp://0.0.0.0:14883 -zmqpubrawtx=tcp://0.0.0.0:14884

regtest=1
rpcallowip=0.0.0.0/0