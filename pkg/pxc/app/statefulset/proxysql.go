package statefulset

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/percona/percona-xtradb-cluster-operator/pkg/apis/pxc/v1"
	app "github.com/percona/percona-xtradb-cluster-operator/pkg/pxc/app"
)

const (
	proxyName           = "proxysql"
	proxyDataVolumeName = "proxydata"
)

type Proxy struct {
	sfs     *appsv1.StatefulSet
	labels  map[string]string
	service string
}

func NewProxy(cr *api.PerconaXtraDBCluster) *Proxy {
	sfs := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-" + proxyName,
			Namespace: cr.Namespace,
		},
	}

	labels := map[string]string{
		"app.kubernetes.io/name":       "percona-xtradb-cluster",
		"app.kubernetes.io/instance":   cr.Name,
		"app.kubernetes.io/component":  proxyName,
		"app.kubernetes.io/managed-by": "percona-xtradb-cluster-operator",
		"app.kubernetes.io/part-of":    "percona-xtradb-cluster",
	}

	return &Proxy{
		sfs:     sfs,
		labels:  labels,
		service: cr.Name + "-proxysql-unready",
	}
}

func (c *Proxy) AppContainer(spec *api.PodSpec, secrets string) corev1.Container {
	appc := corev1.Container{
		Name:            proxyName,
		Image:           spec.Image,
		ImagePullPolicy: corev1.PullAlways,
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 3306,
				Name:          "mysql",
			},
			{
				ContainerPort: 6032,
				Name:          "proxyadm",
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      proxyDataVolumeName,
				MountPath: "/var/lib/proxysql",
			},
			{
				Name:      "ssl",
				MountPath: "/etc/proxysql/ssl",
			},
			{
				Name:      "ssl-internal",
				MountPath: "/etc/proxysql/ssl-internal",
			},
		},
		Env: []corev1.EnvVar{
			{
				Name:  "PXC_SERVICE",
				Value: c.labels["app.kubernetes.io/instance"] + "-pxc",
			},
			{
				Name: "MYSQL_ROOT_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: app.SecretKeySelector(secrets, "root"),
				},
			},
			{
				Name:  "PROXY_ADMIN_USER",
				Value: "proxyadmin",
			},
			{
				Name: "PROXY_ADMIN_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: app.SecretKeySelector(secrets, "proxyadmin"),
				},
			},
			{
				Name: "MONITOR_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: app.SecretKeySelector(secrets, "monitor"),
				},
			},
		},
	}

	return appc
}

func (c *Proxy) SidecarContainers(spec *api.PodSpec, secrets string) []corev1.Container {
	return []corev1.Container{
		{
			Name:            "pxc-monit",
			Image:           spec.Image,
			ImagePullPolicy: corev1.PullAlways,
			Args: []string{
				"/usr/bin/peer-list",
				"-on-change=/usr/bin/add_pxc_nodes.sh",
				"-service=$(PXC_SERVICE)",
			},
			Env: []corev1.EnvVar{
				{
					Name:  "PXC_SERVICE",
					Value: c.labels["app.kubernetes.io/instance"] + "-pxc",
				},
				{
					Name: "MYSQL_ROOT_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: app.SecretKeySelector(secrets, "root"),
					},
				},
				{
					Name:  "PROXY_ADMIN_USER",
					Value: "proxyadmin",
				},
				{
					Name: "PROXY_ADMIN_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: app.SecretKeySelector(secrets, "proxyadmin"),
					},
				},
				{
					Name: "MONITOR_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: app.SecretKeySelector(secrets, "monitor"),
					},
				},
			},
		},

		{
			Name:            "proxysql-monit",
			Image:           spec.Image,
			ImagePullPolicy: corev1.PullAlways,
			Args: []string{
				"/usr/bin/peer-list",
				"-on-change=/usr/bin/add_proxysql_nodes.sh",
				"-service=$(PROXYSQL_SERVICE)",
			},
			Env: []corev1.EnvVar{
				{
					Name:  "PROXYSQL_SERVICE",
					Value: c.labels["app.kubernetes.io/instance"] + "-proxysql-unready",
				},
				{
					Name: "MYSQL_ROOT_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: app.SecretKeySelector(secrets, "root"),
					},
				},
				{
					Name:  "PROXY_ADMIN_USER",
					Value: "proxyadmin",
				},
				{
					Name: "PROXY_ADMIN_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: app.SecretKeySelector(secrets, "proxyadmin"),
					},
				},
				{
					Name: "MONITOR_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: app.SecretKeySelector(secrets, "monitor"),
					},
				},
			},
		},
	}
}

func (c *Proxy) PMMContainer(spec *api.PMMSpec, secrets string, v120OrGreater bool) corev1.Container {
	ct := app.PMMClient(spec, secrets, v120OrGreater)

	pmmEnvs := []corev1.EnvVar{
		{
			Name:  "DB_TYPE",
			Value: "proxysql",
		},
		{
			Name:  "MONITOR_USER",
			Value: "monitor",
		},
		{
			Name: "MONITOR_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: app.SecretKeySelector(secrets, "monitor"),
			},
		},
	}

	dbEnvs := []corev1.EnvVar{
		{
			Name:  "DB_USER",
			Value: "monitor",
		},
		{
			Name: "DB_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: app.SecretKeySelector(secrets, "monitor"),
			},
		},
		{
			Name:  "DB_CLUSTER",
			Value: app.Name,
		},
		{
			Name:  "DB_HOST",
			Value: "localhost",
		},
		{
			Name:  "DB_PORT",
			Value: "6032",
		},
	}

	dbArgsEnv := []corev1.EnvVar{
		{
			Name:  "DB_ARGS",
			Value: "--dsn $(MONITOR_USER):$(MONITOR_PASSWORD)@tcp(localhost:6032)/",
		},
	}

	ct.Env = append(ct.Env, pmmEnvs...)
	switch v120OrGreater {
	case true:
		ct.Env = append(ct.Env, dbEnvs...)
	default:
		ct.Env = append(ct.Env, dbArgsEnv...)
	}

	return ct
}

func (c *Proxy) Resources(spec *api.PodResources) (corev1.ResourceRequirements, error) {
	return app.CreateResources(spec)
}

func (c *Proxy) Volumes(podSpec *api.PodSpec) *api.Volume {
	vol := app.Volumes(podSpec, proxyDataVolumeName)
	vol.Volumes = append(
		vol.Volumes,
		app.GetSecretVolumes("ssl-internal", podSpec.SSLInternalSecretName, true),
		app.GetSecretVolumes("ssl", podSpec.SSLSecretName, podSpec.AllowUnsafeConfig))
	return vol
}

func (c *Proxy) StatefulSet() *appsv1.StatefulSet {
	return c.sfs
}

func (c *Proxy) Labels() map[string]string {
	return c.labels
}

func (c *Proxy) Service() string {
	return c.service
}
