#!/usr/bin/env bash
# Makes Secret rustrak, which the error tracker (infrastructure/rustrak.yaml)
# reads, once: the session key, the first owner's sign-in and the API token
# CI uploads source maps with. A Secret that already exists is left as it is,
# because a new session key signs everyone out and the owner is only created
# while the database has no users. Nothing secret is printed.
#
#   deploy/k8s/scripts/rustrak.sh owner@example.com
set -euo pipefail

email="${1:?usage: rustrak.sh <owner email>}"

kubectl get namespace rustrak >/dev/null 2>&1 || kubectl create namespace rustrak
kubectl label namespace rustrak dimmi.com/protected=true --overwrite >/dev/null

if kubectl -n rustrak get secret rustrak >/dev/null 2>&1; then
	echo "secret rustrak already exists; left as it is"
	exit 0
fi

kubectl -n rustrak create secret generic rustrak \
	--from-literal=SESSION_SECRET_KEY="$(openssl rand -hex 32)" \
	--from-literal=CREATE_SUPERUSER="${email}:$(openssl rand -hex 16)" \
	--from-literal=CI_TOKEN="$(openssl rand -hex 20)" >/dev/null
kubectl -n rustrak label secret rustrak dimmi.com/protected=true >/dev/null

echo "secret rustrak created"
