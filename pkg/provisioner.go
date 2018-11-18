package pkg

import (
	"path"

	"github.com/kubernetes-sigs/sig-storage-lib-external-provisioner/controller"
	"github.com/mistifyio/go-zfs"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	datasetAnnotation = Namespace + "/dataset"
)

type zfsProvisioner struct {
	client kubernetes.Interface
}

func NewZfsProvisioner(client kubernetes.Interface) (controller.Provisioner, error) {
	return &zfsProvisioner{client}, nil
}

func (p *zfsProvisioner) Provision(options controller.VolumeOptions) (*v1.PersistentVolume, error) {
	cfg, err := LoadConfig(options)
	if err != nil {
		return nil, errors.Wrap(err, "error loading config")
	}
	err = validateParentDataset(cfg.ParentDataset)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid parent dataset %s", cfg.ParentDataset)
	}

	datasetFullName := path.Join(cfg.ParentDataset, cfg.Dataset)

	dataset, err := zfs.GetDataset(datasetFullName)
	if err != nil {
		dataset, err = zfs.CreateFilesystem(datasetFullName, map[string]string{})
		if err != nil {
			return nil, errors.Wrapf(err, "error creating zfs dataset %s", datasetFullName)
		}
	}

	// todo: validate dataset mount point

	err = p.setZfsProperties(dataset, options.PVName, cfg)
	if err != nil {
		return nil, errors.Wrapf(err, "error setting properties for zfs dataset %s", datasetFullName)
	}

	var nodeAffinity *v1.VolumeNodeAffinity
	pvSource := v1.PersistentVolumeSource{}

	if !cfg.Local {
		pvSource.NFS = &v1.NFSVolumeSource{
			Server:   cfg.NFSServer,
			Path:     dataset.Mountpoint,
			ReadOnly: false,
		}
	} else {
		pvSource.HostPath = &v1.HostPathVolumeSource{
			Path: dataset.Mountpoint,
		}

		// todo: evaluate if it should be mandatory for local=true
		if node := options.SelectedNode.Name; node != "" {
			nodeAffinity = &v1.VolumeNodeAffinity{
				Required: &v1.NodeSelector{
					NodeSelectorTerms: []v1.NodeSelectorTerm{
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								{
									Key:      "kubernetes.io/hostname",
									Operator: v1.NodeSelectorOpIn,
									Values:   []string{node},
								},
							},
						},
					},
				},
			}
		}
	}

	return &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: options.PVName,
			Annotations: map[string]string{
				datasetAnnotation: cfg.Dataset,
			},
		},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeReclaimPolicy: options.PersistentVolumeReclaimPolicy,
			AccessModes:                   options.PVC.Spec.AccessModes,
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)],
			},
			PersistentVolumeSource: pvSource,
			NodeAffinity:           nodeAffinity,
		},
	}, nil
}

func (p *zfsProvisioner) Delete(volume *v1.PersistentVolume) error {
	// todo evaluate to move parent dataset from storageclass config to provisioner global config for safety
	// in theory, it's unsafe to take full dataset name from PV annotation as it could be manipulated by namespace user
	storageClass, err := p.client.StorageV1().StorageClasses().Get(volume.Spec.StorageClassName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "error while getting storage class %s", volume.Spec.StorageClassName)
	}

	// todo replace using loadConfig instead manually parsing params
	if keepDataset := storageClass.Parameters[keepDatasetParam]; keepDataset == "true" {
		return nil
	}

	parentDataset := storageClass.Parameters[parentDatasetParam]
	if parentDataset == "" {
		return errors.Errorf("can't delete volume without parent dataset annotation")
	}

	dsName := path.Join(parentDataset, volume.Name)
	ds, err := zfs.GetDataset(dsName)
	if err != nil {
		return errors.Wrapf(err, "error getting dataset %s for volume %s", dsName, volume.Name)
	}

	// todo evaluate additional flags to destroy
	return ds.Destroy(0)
}

func (p *zfsProvisioner) setZfsProperties(ds *zfs.Dataset, pvName string, cfg *Config) error {
	props := map[string]string{
		Namespace + ":pv": pvName,
		"sharenfs":        cfg.NFSOptions,
		"refreservation":  cfg.Requests,
		"refquota":        cfg.Limits,
	}

	for _, val := range cfg.Snapshots {
		if val == "all" {
			props["com.sun:auto-snapshot"] = "true"
		} else {
			props["com.sun:auto-snapshot:"+val] = "true"
		}
	}

	for key, val := range props {
		err := ds.SetProperty(key, val)
		if err != nil {
			return errors.Wrapf(err, "error setting property %s=%s", key, val)
		}
	}

	return nil
}

func validateParentDataset(name string) error {
	ds, err := zfs.GetDataset(name)
	if err != nil {
		return err
	}
	if ds.Type != "filesystem" {
		return errors.Errorf("zfs parent dataset %s must be a filesystem", name)
	}

	return nil
}
