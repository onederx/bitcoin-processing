#!/bin/bash -e

psql -v ON_ERROR_STOP=1 -f /create-user-and-db.sql
psql -v ON_ERROR_STOP=1 --username bitcoin_processing -f /init-db.sql