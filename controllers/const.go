package controllers

import kubeedgev1alpha1 "kubesphere.io/edge-watcher/api/v1alpha1"

var (
	ownerKey = ".metadata.controller"
	apiGVStr = kubeedgev1alpha1.GroupVersion.String()
)
