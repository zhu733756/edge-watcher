#!/bin/bash

NodeSelectorPatchJson='{"spec":{"template":{"spec":{"nodeSelector":{"node-role.kubernetes.io/master": "","node-role.kubernetes.io/worker": ""}}}}}'

NoShedulePatchJson='{"spec":{"template":{"spec":{"affinity":{"nodeAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":{"nodeSelectorTerms":[{"matchExpressions":[{"key":"node-role.kubernetes.io/edge","operator":"DoesNotExist"}]}]}}}}}}}'

edgenode="edgenode"

namespaces=($(kubectl get pods -A -o wide |grep $edgenode | awk '{print $1}' ))
pods=($(kubectl get pods -A -o wide |grep $edgenode | awk '{print $2}' ))
length=${#namespaces[@]}

for((i=0;i<$length;i++));  
do
 	ns=${namespaces[$i]}
 	pod=${pods[$i]}
 	resources=$(kubectl -n $ns describe pod $pod | grep "Controlled By" |awk '{print $3}')
 	echo "Patching for ns:"${namespaces[$i]}",resources:"$resources
 	kubectl -n $ns patch $resources --type merge --patch "$NoShedulePatchJson"
 	sleep 1
done
