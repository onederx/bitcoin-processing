#!/bin/bash

where="$1"
amount="$2"

if [ -z "$where" ] || [ -z "$amount" ] ; then
    echo "Usage: $0 ADDRESS AMOUNT"
    exit 1
fi

node2/bitcoin-cli sendtoaddress $where $amount