#!/bin/bash

HOST="127.0.0.1"
PORT=4000

timeout 2 bash -c "</dev/tcp/$HOST/$PORT"
if [ $? -eq 0 ]; then
    exit 0 
else
    exit 1   
fi
