// Copyright 2017 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kubernetes

import (
	"context"

	"github.com/tsuru/config"
	tsuruv1 "github.com/tsuru/tsuru/provision/kubernetes/pkg/apis/tsuru/v1"
	"github.com/tsuru/tsuru/provision/provisiontest"
	"github.com/tsuru/tsuru/servicemanager"
	volumeTypes "github.com/tsuru/tsuru/types/volume"
	check "gopkg.in/check.v1"
	apiv1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *S) TestCreateVolumesForAppPlugin(c *check.C) {
	config.Set("volume-plans:p1:kubernetes:plugin", "nfs")
	defer config.Unset("volume-plans")
	a := provisiontest.NewFakeApp("myapp", "python", 0)
	err := s.p.Provision(context.TODO(), a)
	c.Assert(err, check.IsNil)
	v := volumeTypes.Volume{
		Name: "v1",
		Opts: map[string]string{
			"path":         "/exports",
			"server":       "192.168.1.1",
			"capacity":     "20Gi",
			"access-modes": string(apiv1.ReadWriteMany),
		},
		Plan:      volumeTypes.VolumePlan{Name: "p1"},
		Pool:      "test-default",
		TeamOwner: "admin",
	}
	err = servicemanager.Volume.Create(context.TODO(), &v)
	c.Assert(err, check.IsNil)
	err = servicemanager.Volume.BindApp(context.TODO(), &volumeTypes.BindOpts{
		Volume:     &v,
		AppName:    a.Name,
		MountPoint: "/mnt",
		ReadOnly:   false,
	})
	c.Assert(err, check.IsNil)
	err = servicemanager.Volume.BindApp(context.TODO(), &volumeTypes.BindOpts{
		Volume:     &v,
		AppName:    a.Name,
		MountPoint: "/mnt2",
		ReadOnly:   false,
	})
	c.Assert(err, check.IsNil)
	err = s.p.Provision(context.TODO(), provisiontest.NewFakeApp("otherapp", "python", 0))
	c.Assert(err, check.IsNil)
	err = servicemanager.Volume.BindApp(context.TODO(), &volumeTypes.BindOpts{
		Volume:     &v,
		AppName:    "otherapp",
		MountPoint: "/mnt",
		ReadOnly:   false,
	})
	c.Assert(err, check.IsNil)
	volumes, mounts, err := createVolumesForApp(context.TODO(), s.clusterClient, a)
	c.Assert(err, check.IsNil)
	expectedVolume := []apiv1.Volume{{
		Name: volumeName(v.Name),
		VolumeSource: apiv1.VolumeSource{
			PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
				ClaimName: volumeClaimName(v.Name),
				ReadOnly:  false,
			},
		},
	}}
	expectedMount := []apiv1.VolumeMount{
		{
			Name:      volumeName(v.Name),
			MountPath: "/mnt",
			ReadOnly:  false,
		},
		{
			Name:      volumeName(v.Name),
			MountPath: "/mnt2",
			ReadOnly:  false,
		},
	}
	c.Check(volumes, check.DeepEquals, expectedVolume)
	c.Check(mounts, check.DeepEquals, expectedMount)
	pv, err := s.client.CoreV1().PersistentVolumes().Get(context.TODO(), volumeName(v.Name), metav1.GetOptions{})
	c.Assert(err, check.IsNil)
	expectedCap, err := resource.ParseQuantity("20Gi")
	c.Assert(err, check.IsNil)
	c.Assert(pv, check.DeepEquals, &apiv1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: volumeName(v.Name),
			Labels: map[string]string{
				"tsuru.io/volume-name": "v1",
				"tsuru.io/volume-pool": "test-default",
				"tsuru.io/volume-plan": "p1",
				"tsuru.io/volume-team": "admin",
				"tsuru.io/is-tsuru":    "true",
			},
		},
		Spec: apiv1.PersistentVolumeSpec{
			PersistentVolumeSource: apiv1.PersistentVolumeSource{
				NFS: &apiv1.NFSVolumeSource{
					Path:   "/exports",
					Server: "192.168.1.1",
				},
			},
			AccessModes: []apiv1.PersistentVolumeAccessMode{apiv1.ReadWriteMany},
			Capacity: apiv1.ResourceList{
				apiv1.ResourceStorage: expectedCap,
			},
		},
	})
	ns, err := s.client.AppNamespace(context.TODO(), a)
	c.Assert(err, check.IsNil)
	pvc, err := s.client.CoreV1().PersistentVolumeClaims(ns).Get(context.TODO(), volumeClaimName(v.Name), metav1.GetOptions{})
	c.Assert(err, check.IsNil)
	emptyStr := ""
	c.Assert(pvc, check.DeepEquals, &apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: volumeClaimName(v.Name),
			Labels: map[string]string{
				"tsuru.io/volume-name": "v1",
				"tsuru.io/volume-pool": "test-default",
				"tsuru.io/volume-plan": "p1",
				"tsuru.io/volume-team": "admin",
				"tsuru.io/is-tsuru":    "true",
			},
			Namespace: ns,
		},
		Spec: apiv1.PersistentVolumeClaimSpec{
			AccessModes: []apiv1.PersistentVolumeAccessMode{apiv1.ReadWriteMany},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"tsuru.io/volume-name": "v1"},
			},
			VolumeName:       volumeName(v.Name),
			StorageClassName: &emptyStr,
			Resources: apiv1.ResourceRequirements{
				Requests: apiv1.ResourceList{
					apiv1.ResourceStorage: expectedCap,
				},
			},
		},
	})
	volumes, mounts, err = createVolumesForApp(context.TODO(), s.clusterClient, a)
	c.Assert(err, check.IsNil)
	c.Assert(volumes, check.DeepEquals, expectedVolume)
	c.Assert(mounts, check.DeepEquals, expectedMount)
}

func (s *S) TestCreateVolumesForAppPluginNonPersistentEmptyDir(c *check.C) {
	config.Set("volume-plans:p1:kubernetes:plugin", "emptyDir")
	defer config.Unset("volume-plans")
	a := provisiontest.NewFakeApp("myapp", "python", 0)
	err := s.p.Provision(context.TODO(), a)
	c.Assert(err, check.IsNil)
	v := volumeTypes.Volume{
		Name: "v1",
		Opts: map[string]string{
			"medium": "Memory",
		},
		Plan:      volumeTypes.VolumePlan{Name: "p1"},
		Pool:      "test-default",
		TeamOwner: "admin",
	}
	err = servicemanager.Volume.Create(context.TODO(), &v)
	c.Assert(err, check.IsNil)
	err = servicemanager.Volume.BindApp(context.TODO(), &volumeTypes.BindOpts{
		Volume:     &v,
		AppName:    a.Name,
		MountPoint: "/mnt",
		ReadOnly:   false,
	})
	c.Assert(err, check.IsNil)
	err = servicemanager.Volume.BindApp(context.TODO(), &volumeTypes.BindOpts{
		Volume:     &v,
		AppName:    a.Name,
		MountPoint: "/mnt2",
		ReadOnly:   false,
	})
	c.Assert(err, check.IsNil)
	err = servicemanager.Volume.BindApp(context.TODO(), &volumeTypes.BindOpts{
		Volume:     &v,
		AppName:    "otherapp",
		MountPoint: "/mnt",
		ReadOnly:   false,
	})
	c.Assert(err, check.IsNil)
	volumes, mounts, err := createVolumesForApp(context.TODO(), s.clusterClient, a)
	c.Assert(err, check.IsNil)
	expectedVolume := []apiv1.Volume{{
		Name: volumeName(v.Name),
		VolumeSource: apiv1.VolumeSource{
			EmptyDir: &apiv1.EmptyDirVolumeSource{
				Medium: apiv1.StorageMediumMemory,
			},
		},
	}}
	expectedMount := []apiv1.VolumeMount{
		{
			Name:      volumeName(v.Name),
			MountPath: "/mnt",
			ReadOnly:  false,
		},
		{
			Name:      volumeName(v.Name),
			MountPath: "/mnt2",
			ReadOnly:  false,
		},
	}
	c.Assert(volumes, check.DeepEquals, expectedVolume)
	c.Assert(mounts, check.DeepEquals, expectedMount)
	_, err = s.client.CoreV1().PersistentVolumes().Get(context.TODO(), volumeName(v.Name), metav1.GetOptions{})
	c.Assert(k8sErrors.IsNotFound(err), check.Equals, true)
	ns, err := s.client.AppNamespace(context.TODO(), a)
	c.Assert(err, check.IsNil)
	_, err = s.client.CoreV1().PersistentVolumeClaims(ns).Get(context.TODO(), volumeClaimName(v.Name), metav1.GetOptions{})
	c.Assert(k8sErrors.IsNotFound(err), check.Equals, true)
	volumes, mounts, err = createVolumesForApp(context.TODO(), s.clusterClient, a)
	c.Assert(err, check.IsNil)
	c.Assert(volumes, check.DeepEquals, expectedVolume)
	c.Assert(mounts, check.DeepEquals, expectedMount)
}

func (s *S) TestCreateVolumesForAppPluginNonPersistentEphemeral(c *check.C) {
	config.Set("volume-plans:p1:kubernetes:plugin", "ephemeral")
	config.Set("volume-plans:p1:kubernetes:storage-class", "my-storage-class")
	config.Set("volume-plans:p1:kubernetes:access-modes", "ReadWriteOnce")
	defer config.Unset("volume-plans")
	a := provisiontest.NewFakeApp("myapp", "python", 0)
	err := s.p.Provision(context.TODO(), a)
	c.Assert(err, check.IsNil)
	v := volumeTypes.Volume{
		Name: "v1",
		Opts: map[string]string{
			"capacity": "10Gi",
		},
		Plan: volumeTypes.VolumePlan{Name: "p1", Opts: map[string]interface{}{
			"storage-class": "my-storage-class",
		}},
		Pool:      "test-default",
		TeamOwner: "admin",
	}
	err = servicemanager.Volume.Create(context.TODO(), &v)
	c.Assert(err, check.IsNil)
	err = servicemanager.Volume.BindApp(context.TODO(), &volumeTypes.BindOpts{
		Volume:     &v,
		AppName:    a.Name,
		MountPoint: "/mnt",
		ReadOnly:   false,
	})
	c.Assert(err, check.IsNil)
	err = servicemanager.Volume.BindApp(context.TODO(), &volumeTypes.BindOpts{
		Volume:     &v,
		AppName:    a.Name,
		MountPoint: "/mnt2",
		ReadOnly:   false,
	})
	c.Assert(err, check.IsNil)
	err = servicemanager.Volume.BindApp(context.TODO(), &volumeTypes.BindOpts{
		Volume:     &v,
		AppName:    "otherapp",
		MountPoint: "/mnt",
		ReadOnly:   false,
	})
	c.Assert(err, check.IsNil)
	volumes, mounts, err := createVolumesForApp(context.TODO(), s.clusterClient, a)
	c.Assert(err, check.IsNil)
	expectedStorageClass := "my-storage-class"
	expectedCap, _ := resource.ParseQuantity("10Gi")
	expectedVolume := []apiv1.Volume{{
		Name: volumeName(v.Name),
		VolumeSource: apiv1.VolumeSource{
			Ephemeral: &apiv1.EphemeralVolumeSource{
				VolumeClaimTemplate: &apiv1.PersistentVolumeClaimTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"tsuru.io/volume-name": "v1",
							"tsuru.io/volume-pool": "test-default",
							"tsuru.io/volume-plan": "p1",
							"tsuru.io/volume-team": "admin",
							"tsuru.io/is-tsuru":    "true",
						},
					},
					Spec: apiv1.PersistentVolumeClaimSpec{
						StorageClassName: &expectedStorageClass,
						AccessModes:      []apiv1.PersistentVolumeAccessMode{apiv1.ReadWriteOnce},
						Resources: apiv1.ResourceRequirements{
							Requests: apiv1.ResourceList{
								apiv1.ResourceStorage: expectedCap,
							},
						},
					},
				},
			},
		},
	}}
	expectedMount := []apiv1.VolumeMount{
		{
			Name:      volumeName(v.Name),
			MountPath: "/mnt",
			ReadOnly:  false,
		},
		{
			Name:      volumeName(v.Name),
			MountPath: "/mnt2",
			ReadOnly:  false,
		},
	}
	c.Assert(volumes, check.DeepEquals, expectedVolume)
	c.Assert(mounts, check.DeepEquals, expectedMount)
	_, err = s.client.CoreV1().PersistentVolumes().Get(context.TODO(), volumeName(v.Name), metav1.GetOptions{})
	c.Assert(k8sErrors.IsNotFound(err), check.Equals, true)
	ns, err := s.client.AppNamespace(context.TODO(), a)
	c.Assert(err, check.IsNil)
	_, err = s.client.CoreV1().PersistentVolumeClaims(ns).Get(context.TODO(), volumeClaimName(v.Name), metav1.GetOptions{})
	c.Assert(k8sErrors.IsNotFound(err), check.Equals, true)
	volumes, mounts, err = createVolumesForApp(context.TODO(), s.clusterClient, a)
	c.Assert(err, check.IsNil)
	c.Assert(volumes, check.DeepEquals, expectedVolume)
	c.Assert(mounts, check.DeepEquals, expectedMount)
}

func (s *S) TestCreateVolumesForAppStorageClass(c *check.C) {
	config.Set("volume-plans:p1:kubernetes:storage-class", "my-class")
	config.Set("volume-plans:p1:kubernetes:capacity", "20Gi")
	config.Set("volume-plans:p1:kubernetes:access-modes", "ReadWriteMany")
	defer config.Unset("volume-plans")
	a := provisiontest.NewFakeApp("myapp", "python", 0)
	err := s.p.Provision(context.TODO(), a)
	c.Assert(err, check.IsNil)
	v := volumeTypes.Volume{
		Name:      "v1",
		Plan:      volumeTypes.VolumePlan{Name: "p1"},
		Pool:      "test-default",
		TeamOwner: "admin",
	}
	err = servicemanager.Volume.Create(context.TODO(), &v)
	c.Assert(err, check.IsNil)
	err = servicemanager.Volume.BindApp(context.TODO(), &volumeTypes.BindOpts{
		Volume:     &v,
		AppName:    a.Name,
		MountPoint: "/mnt",
		ReadOnly:   false,
	})
	c.Assert(err, check.IsNil)
	volumes, mounts, err := createVolumesForApp(context.TODO(), s.clusterClient, a)
	c.Assert(err, check.IsNil)
	expectedVolume := []apiv1.Volume{{
		Name: volumeName(v.Name),
		VolumeSource: apiv1.VolumeSource{
			PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
				ClaimName: volumeClaimName(v.Name),
				ReadOnly:  false,
			},
		},
	}}
	expectedMount := []apiv1.VolumeMount{{
		Name:      volumeName(v.Name),
		MountPath: "/mnt",
		ReadOnly:  false,
	}}
	c.Assert(volumes, check.DeepEquals, expectedVolume)
	c.Assert(mounts, check.DeepEquals, expectedMount)
	_, err = s.client.CoreV1().PersistentVolumes().Get(context.TODO(), volumeName(v.Name), metav1.GetOptions{})
	c.Assert(err, check.ErrorMatches, "persistentvolumes \"v1-tsuru\" not found")
	expectedClass := "my-class"
	expectedCap, err := resource.ParseQuantity("20Gi")
	c.Assert(err, check.IsNil)
	ns, err := s.client.AppNamespace(context.TODO(), a)
	c.Assert(err, check.IsNil)
	pvc, err := s.client.CoreV1().PersistentVolumeClaims(ns).Get(context.TODO(), volumeClaimName(v.Name), metav1.GetOptions{})
	c.Assert(err, check.IsNil)
	c.Assert(pvc, check.DeepEquals, &apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: volumeClaimName(v.Name),
			Labels: map[string]string{
				"tsuru.io/volume-name": "v1",
				"tsuru.io/volume-pool": "test-default",
				"tsuru.io/volume-plan": "p1",
				"tsuru.io/volume-team": "admin",
				"tsuru.io/is-tsuru":    "true",
			},
			Namespace: ns,
		},
		Spec: apiv1.PersistentVolumeClaimSpec{
			AccessModes:      []apiv1.PersistentVolumeAccessMode{apiv1.ReadWriteMany},
			StorageClassName: &expectedClass,
			Resources: apiv1.ResourceRequirements{
				Requests: apiv1.ResourceList{
					apiv1.ResourceStorage: expectedCap,
				},
			},
		},
	})
	volumes, mounts, err = createVolumesForApp(context.TODO(), s.clusterClient, a)
	c.Assert(err, check.IsNil)
	c.Assert(volumes, check.DeepEquals, expectedVolume)
	c.Assert(mounts, check.DeepEquals, expectedMount)
}

func (s *S) TestCreateVolumeAppNamespace(c *check.C) {
	config.Set("volume-plans:p1:kubernetes:plugin", "nfs")
	defer config.Unset("volume-plans")
	a := provisiontest.NewFakeApp("myapp", "python", 0)
	err := s.p.Provision(context.TODO(), a)
	c.Assert(err, check.IsNil)
	appCR := tsuruv1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: a.Name,
		},
		Spec: tsuruv1.AppSpec{
			NamespaceName: "custom-namespace",
		},
	}
	_, err = s.client.TsuruV1().Apps(s.client.Namespace()).Update(context.TODO(), &appCR, metav1.UpdateOptions{})
	c.Assert(err, check.IsNil)
	v := volumeTypes.Volume{
		Name: "v1",
		Opts: map[string]string{
			"path":         "/exports",
			"server":       "192.168.1.1",
			"capacity":     "20Gi",
			"access-modes": string(apiv1.ReadWriteMany),
		},
		Plan:      volumeTypes.VolumePlan{Name: "p1"},
		Pool:      "test-default",
		TeamOwner: "admin",
	}
	err = servicemanager.Volume.Create(context.TODO(), &v)
	c.Assert(err, check.IsNil)
	err = servicemanager.Volume.BindApp(context.TODO(), &volumeTypes.BindOpts{
		Volume:     &v,
		AppName:    a.Name,
		MountPoint: "/mnt",
		ReadOnly:   false,
	})
	c.Assert(err, check.IsNil)
	_, _, err = createVolumesForApp(context.TODO(), s.clusterClient, a)
	c.Assert(err, check.IsNil)
	pvc, err := s.client.CoreV1().PersistentVolumeClaims("custom-namespace").Get(context.TODO(), volumeClaimName(v.Name), metav1.GetOptions{})
	c.Assert(err, check.IsNil)
	c.Assert(pvc.ObjectMeta, check.DeepEquals, metav1.ObjectMeta{
		Name: volumeClaimName(v.Name),
		Labels: map[string]string{
			"tsuru.io/volume-name": "v1",
			"tsuru.io/volume-pool": "test-default",
			"tsuru.io/volume-plan": "p1",
			"tsuru.io/volume-team": "admin",
			"tsuru.io/is-tsuru":    "true",
		},
		Namespace: "custom-namespace",
	})
}

func (s *S) TestCreateVolumeMultipleNamespacesFail(c *check.C) {
	config.Set("kubernetes:use-pool-namespaces", true)
	defer config.Unset("kubernetes:use-pool-namespaces")
	config.Set("volume-plans:p1:kubernetes:plugin", "nfs")
	defer config.Unset("volume-plans")
	a := provisiontest.NewFakeApp("myapp", "python", 0)
	err := s.p.Provision(context.TODO(), a)
	c.Assert(err, check.IsNil)
	v := volumeTypes.Volume{
		Name: "v1",
		Opts: map[string]string{
			"path":         "/exports",
			"server":       "192.168.1.1",
			"capacity":     "20Gi",
			"access-modes": string(apiv1.ReadWriteMany),
		},
		Plan:      volumeTypes.VolumePlan{Name: "p1"},
		Pool:      "test-default",
		TeamOwner: "admin",
	}
	err = servicemanager.Volume.Create(context.TODO(), &v)
	c.Assert(err, check.IsNil)
	err = servicemanager.Volume.BindApp(context.TODO(), &volumeTypes.BindOpts{
		Volume:     &v,
		AppName:    a.Name,
		MountPoint: "/mnt",
		ReadOnly:   false,
	})
	c.Assert(err, check.IsNil)
	err = s.p.Provision(context.TODO(), provisiontest.NewFakeAppWithPool("otherapp", "python", "otherpool", 0))
	c.Assert(err, check.IsNil)
	err = servicemanager.Volume.BindApp(context.TODO(), &volumeTypes.BindOpts{
		Volume:     &v,
		AppName:    "otherapp",
		MountPoint: "/mnt",
		ReadOnly:   false,
	})
	c.Assert(err, check.IsNil)
	_, _, err = createVolumesForApp(context.TODO(), s.clusterClient, a)
	c.Assert(err, check.ErrorMatches, `multiple namespaces for volume not allowed: "tsuru-otherpool" and "tsuru-test-default"`)
}

func (s *S) TestDeleteVolume(c *check.C) {
	config.Set("volume-plans:p1:kubernetes:plugin", "nfs")
	defer config.Unset("volume-plans")
	a := provisiontest.NewFakeApp("myapp", "python", 0)
	err := s.p.Provision(context.TODO(), a)
	c.Assert(err, check.IsNil)
	v := volumeTypes.Volume{
		Name: "v1",
		Opts: map[string]string{
			"path":         "/exports",
			"server":       "192.168.1.1",
			"capacity":     "20Gi",
			"access-modes": string(apiv1.ReadWriteMany),
		},
		Plan: volumeTypes.VolumePlan{Name: "p1", Opts: map[string]interface{}{
			"storage-class": "myown",
		}},
		Pool:      "test-default",
		TeamOwner: "admin",
	}
	err = servicemanager.Volume.Create(context.TODO(), &v)
	c.Assert(err, check.IsNil)
	err = servicemanager.Volume.BindApp(context.TODO(), &volumeTypes.BindOpts{
		Volume:     &v,
		AppName:    a.Name,
		MountPoint: "/mnt",
		ReadOnly:   false,
	})
	c.Assert(err, check.IsNil)
	_, _, err = createVolumesForApp(context.TODO(), s.clusterClient, a)
	c.Assert(err, check.IsNil)
	ns, err := s.client.AppNamespace(context.TODO(), a)
	c.Assert(err, check.IsNil)
	err = deleteVolume(context.TODO(), s.clusterClient, "v1")
	c.Assert(err, check.IsNil)
	_, err = s.client.CoreV1().PersistentVolumes().Get(context.TODO(), volumeName(v.Name), metav1.GetOptions{})
	c.Assert(k8sErrors.IsNotFound(err), check.Equals, true)
	_, err = s.client.CoreV1().PersistentVolumeClaims(ns).Get(context.TODO(), volumeClaimName(v.Name), metav1.GetOptions{})
	c.Assert(k8sErrors.IsNotFound(err), check.Equals, true)
}

func (s *S) TestVolumeExists(c *check.C) {
	config.Set("volume-plans:p1:kubernetes:plugin", "nfs")
	defer config.Unset("volume-plans")
	exists, err := volumeExists(context.TODO(), s.clusterClient, "v1")
	c.Assert(err, check.IsNil)
	c.Assert(exists, check.Equals, false)
	a := provisiontest.NewFakeApp("myapp", "python", 0)
	err = s.p.Provision(context.TODO(), a)
	c.Assert(err, check.IsNil)
	v := volumeTypes.Volume{
		Name: "v1",
		Opts: map[string]string{
			"path":         "/exports",
			"server":       "192.168.1.1",
			"capacity":     "20Gi",
			"access-modes": string(apiv1.ReadWriteMany),
		},
		Plan: volumeTypes.VolumePlan{Name: "p1", Opts: map[string]interface{}{
			"storage-class": "mystorage-class",
		}},
		Pool:      "test-default",
		TeamOwner: "admin",
	}
	err = servicemanager.Volume.Create(context.TODO(), &v)
	c.Assert(err, check.IsNil)
	err = servicemanager.Volume.BindApp(context.TODO(), &volumeTypes.BindOpts{
		Volume:     &v,
		AppName:    a.Name,
		MountPoint: "/mnt",
		ReadOnly:   false,
	})
	c.Assert(err, check.IsNil)
	_, _, err = createVolumesForApp(context.TODO(), s.clusterClient, a)
	c.Assert(err, check.IsNil)
	exists, err = volumeExists(context.TODO(), s.clusterClient, "v1")
	c.Assert(err, check.IsNil)
	c.Assert(exists, check.Equals, true)
}
