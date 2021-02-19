package operator

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubeedgev1alpha1 "kubesphere.io/edge-watcher/api/v1alpha1"
)

func MakeRBACObjects(itName, itNamespace string) (rbacv1.ClusterRole, corev1.ServiceAccount, rbacv1.ClusterRoleBinding) {
	cr := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubesphere:iptables",
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get"},
				APIGroups: []string{""},
				Resources: []string{"pods"},
			},
		},
	}

	sa := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      itName,
			Namespace: itNamespace,
		},
	}

	crb := rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubesphere:iptables",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      itName,
				Namespace: itNamespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "kubesphere:iptables",
		},
	}

	return cr, sa, crb
}

func MakeDaemonSet(it kubeedgev1alpha1.IPTables) appsv1.DaemonSet {
	ds := appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      it.Name,
			Namespace: it.Namespace,
			Labels:    it.Labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: it.Labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      it.Name,
					Namespace: it.Namespace,
					Labels:    it.Labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: it.Name,
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: it.Name,
								},
							},
						},
						{
							Name: "host-time",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/etc/localtime",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "iptables",
							Image:           it.Spec.Image,
							ImagePullPolicy: it.Spec.ImagePullPolicy,
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{
										"NET_ADMIN",
									},
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "metrics",
									ContainerPort: 2020,
									Protocol:      "TCP",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config",
									ReadOnly:  true,
									MountPath: "/etc/iptables.conf",
								},
								{
									Name:      "host-time",
									MountPath: "/etc/localtime",
								},
							},
						},
					},
					NodeSelector: it.Spec.NodeSelector,
					Tolerations:  it.Spec.Tolerations,
					HostNetwork:  true,
				},
			},
		},
	}

	return ds
}
