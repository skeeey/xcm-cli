#!/bin/bash

server="18.117.108.103"
token=""

# curl -v -X 'GET' \
#   "http://$server/ping" \
#   -H 'accept: application/json' \
#   -H "Authorization: Bearer $token"


curl -v "http://$server/info"