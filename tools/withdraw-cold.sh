#!/bin/bash

address="$1"
amount="$2"

if [ -z "$address" ] || [ -z "$amount" ] ; then
    echo "Usage: $0 ADDRESS AMOUNT [FEE [FEE_TYPE]]"
    echo "Example: $0 2NDotUj6Y9eMUmyrYDwhi9F8jDaNY7ev7A8 0.5"
    echo "Example: $0 2NDotUj6Y9eMUmyrYDwhi9F8jDaNY7ev7A8 0.5 0.0001 fixed"
    exit 1
fi

fee="$3"
fee_type="$4"

if [ -z "$fee" ] ; then
    echo "Requesting default fee 0.0001"
    fee="0.0001"
fi

if [ -z "$fee_type" ] ; then
    fee_type="fixed"
fi

UUID=`python -c 'import uuid; print uuid.uuid4()'`

echo "Withdraw with id $UUID"

curl -s -X POST http://127.0.0.1:8000/withdraw-to-cold-storage --data '{
    "id": "'$UUID'",
    "address": "'$address'",
    "amount": "'$amount'",
    "fee": "'$fee'",
    "fee_type": "'$fee_type'",
    "metainfo": {"payment": "outgoing"}
}'