#!/bin/bash

server="52.14.76.170"
token=""

# curl -v -X 'GET' \
#   "http://$server/ping" \
#   -H 'accept: application/json' \
#   -H "Authorization: Bearer $token"


curl -v "http://$server/info"