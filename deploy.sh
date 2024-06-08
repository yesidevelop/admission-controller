#!/bin/bash

docker build -t mutating-webhook .
docker tag mutating-webhook:latest <ecr-image-url>
docker push <ecr-image-url>

openssl genrsa -out tls.key 2048
openssl req -new -key tls.key -out tls.csr -subj "/CN=my-webhook-server.default.svc"
openssl x509 -req -extfile <(printf "subjectAltName=DNS:my-webhook-server.default.svc") -in tls.csr -signkey tls.key -out tls.crt

echo "Creating Webhook TLS Secret"
kubectl create secret tls my-webhook-server-tls \
    --cert "tls.crt" \
    --key "tls.key"



# Base 64 Encode

cat tls.crt | base64 | tr -d '\n'


kubectl apply -f webhook.yaml
kubectl apply -f webhook-configuration.yaml