#!/bin/bash

curl -s -X POST http://127.0.0.1:8000/new-address --data '{
    "user": "testuser",
    "id": '$RANDOM'
}' | python -m json.tool