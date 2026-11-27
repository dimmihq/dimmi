#!/usr/bin/env bash
# Makes Secret kimmi, which the session recorder (infrastructure/kimmi.yaml)
# reads, once: the key that signs its sign-in cookies and the first owner's
# sign-in. A Secret that already exists is left as it is, because a new key
# signs everyone out and the owner is only created while the database has no
# users. Nothing secret is printed.
#
#   deploy/k8s/scripts/kimmi.sh owner@example.com
set -euo pipefail

email="${1:?usage: kimmi.sh <owner email>}"

kubectl get namespace kimmi >/dev/null 2>&1 || kubectl create namespace kimmi
kubectl label namespace kimmi dimmi.com/protected=true --overwrite >/dev/null

if kubectl -n kimmi get secret kimmi >/dev/null 2>&1; then
	echo "secret kimmi already exists; left as it is"
	exit 0
fi

kubectl -n kimmi create secret generic kimmi \
	--from-literal=KIMMI_SECRET="$(openssl rand -hex 32)" \
	--from-literal=KIMMI_OWNER="${email}:$(openssl rand -hex 16)" >/dev/null
kubectl -n kimmi label secret kimmi dimmi.com/protected=true >/dev/null

echo "secret kimmi created"
