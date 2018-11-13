package pkg

import (
	"github.com/kubernetes-sigs/sig-storage-lib-external-provisioner/controller"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

const (
	Namespace          = "k8s-zfs.frq.me"
	parentDatasetParam = "parentDataset"
	keepDatasetParam   = "keepDataset"
)

var (
	validDataset = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_\-:.]{0,250}$`)
)

type StorageClassConfig struct {
	ParentDataset    string
	KeepDataset      bool
	Local            bool
	NFSServer        string
	NFSOptions       string
	SnapshotsEnabled bool
	DefaultSnapshots []string
}

type ClaimConfig struct {
	Snapshots []string `mapstructure:"k8s-zfs.frq.me/snapshots"`
	Dataset   string   `mapstructure:"k8s-zfs.frq.me/dataset"`
}

type Config struct {
	ParentDataset string
	Local         bool
	NFSServer     string
	NFSOptions    string
	Snapshots     []string
	Dataset       string
	Requests      string
	Limits        string
}

func LoadConfig(options controller.VolumeOptions) (*Config, error) {
	scCfg := &StorageClassConfig{
		KeepDataset:      true,
		NFSOptions:       "on",
		SnapshotsEnabled: true,
	}
	err := decode(options.Parameters, scCfg)
	if err != nil {
		return nil, errors.Wrap(err, "error decoding StorageClass parameters")
	}

	if !scCfg.Local && (scCfg.NFSServer == "" || scCfg.NFSOptions == "") {
		return nil, errors.New("NFSServer and NFSOptions should be non-empty for non-local ZFS StorageClass")
	}

	if len(scCfg.DefaultSnapshots) == 0 {
		scCfg.DefaultSnapshots = []string{"daily", "weekly", "monthly"}
	}

	pvcCfg := &ClaimConfig{
		Dataset: options.PVName,
	}
	err = decode(options.PVC.Annotations, pvcCfg)

	if !validDataset.MatchString(pvcCfg.Dataset) {
		return nil, errors.Errorf("invalid zfs dataset name: %s", pvcCfg.Dataset)
	}

	if !scCfg.SnapshotsEnabled {
		pvcCfg.Snapshots = []string{}
	} else if len(pvcCfg.Snapshots) == 0 {
		pvcCfg.Snapshots = scCfg.DefaultSnapshots
	}

	requests := options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	requestsBytes := requests.Value()

	limits := options.PVC.Spec.Resources.Limits[v1.ResourceName(v1.ResourceStorage)]
	limitsBytes := limits.Value()

	if requestsBytes == 0 && limitsBytes == 0 {
		return nil, errors.Errorf("at least one of the requests or limits should be non null for %s", options.PVName)
	}

	if requestsBytes == 0 {
		requestsBytes = limitsBytes
	}
	if limitsBytes == 0 {
		limitsBytes = requestsBytes
	}

	return &Config{
		ParentDataset: scCfg.ParentDataset,
		Local:         scCfg.Local,
		NFSServer:     scCfg.NFSServer,
		NFSOptions:    scCfg.NFSOptions,
		Snapshots:     pvcCfg.Snapshots,
		Dataset:       pvcCfg.Dataset,
		Requests:      strconv.FormatInt(requestsBytes, 10),
		Limits:        strconv.FormatInt(limitsBytes, 10),
	}, nil
}

func decode(from, to interface{}) error {
	decoderCfg := &mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			trimmingStringToStringHookFunc(),
			trimmingStringToSliceHookFunc(","),
		),
		WeaklyTypedInput: true,
		Result:           to,
	}
	decoder, err := mapstructure.NewDecoder(decoderCfg)
	if err != nil {
		return errors.Wrap(err, "error creating decoder")
	}

	err = decoder.Decode(from)
	if err != nil {
		return err
	}

	return nil
}

func trimmingStringToStringHookFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Kind,
		t reflect.Kind,
		data interface{}) (interface{}, error) {
		if f != reflect.String || t != reflect.String {
			return data, nil
		}

		return strings.TrimSpace(data.(string)), nil
	}
}

func trimmingStringToSliceHookFunc(sep string) mapstructure.DecodeHookFunc {
	return func(
		f reflect.Kind,
		t reflect.Kind,
		data interface{}) (interface{}, error) {
		if f != reflect.String || t != reflect.Slice {
			return data, nil
		}

		raw := data.(string)
		if raw == "" {
			return []string{}, nil
		}

		parts := strings.Split(raw, sep)
		for idx, val := range parts {
			parts[idx] = strings.TrimSpace(val)
		}

		return parts, nil
	}
}
