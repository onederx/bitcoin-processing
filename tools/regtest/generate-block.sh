#!/bin/bash

N="$1"

if [ -z "$N" ] ; then
    N=1
fi

node2/bitcoin-cli generate $N