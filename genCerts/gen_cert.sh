#!/bin/sh
openssl req -new -key ./genCerts/cert.key -subj "/CN=$1" -sha256 | openssl x509 -req -days 3650 -CA ./genCerts/ca.crt -CAkey ./genCerts/ca.key -set_serial "$2" > ./genCerts/certs/"$1".crt