#!/bin/bash

ROLE=$1
PORT=50000


curl -X POST "http://localhost:${PORT}/role_change?role=${ROLE}" \
     -H "Content-Type: application/json" \
     --max-time 2