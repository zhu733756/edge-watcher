package operator

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	EdgeNodeJoinCmd = "edgenode-join"
)

func MakeEdgeNodeJoinDefaultConfigMap(itNamespace string) corev1.ConfigMap {
	cm := map[string]string{
		"version": "v1.5.0",
		"zone":    "zh",
		"uri":     "",
	}
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      EdgeNodeJoinCmd,
			Namespace: itNamespace,
		},
		Data: cm,
	}
}
