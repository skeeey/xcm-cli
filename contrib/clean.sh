#!/bin/bash

echo ">>> clean the spoke ..."
# kubectl delete klusterlets --all
# sleep 10
# kubectl delete ns open-cluster-management-managed-serviceaccount --ignore-not-found
# kubectl delete ns open-cluster-management --ignore-not-found
# kubectl get ns | grep cluster | grep -v multicluster-controlplane | awk '{print $1}' | xargs kubectl delete ns

echo ">>> clean the control plane ..."
# kubectl --kubeconfig ${CONTROLPLANE_KUBECONFIG} delete managedcluster --all

kubectl -n multicluster-controlplane delete deploy --all
kubectl -n multicluster-controlplane delete secrets --all
