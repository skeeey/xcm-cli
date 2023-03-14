#!/bin/bash

echo ">>> clean the spoke ..."
kubectl delete ns multicluster-controlplane-agent --ignore-not-found

echo ">>> clean the control plane ..."
kubectl -n multicluster-controlplane delete deploy --all
kubectl -n multicluster-controlplane delete secrets --all
