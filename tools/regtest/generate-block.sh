#!/bin/bash

N="$1"

if [ -z "$N" ] ; then
    N=1
fi

node3/bitcoin-cli generate $N