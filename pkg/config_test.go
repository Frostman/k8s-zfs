package pkg_test

import (
	"fmt"
	"github.com/kubernetes-sigs/sig-storage-lib-external-provisioner/controller"
	"github.com/stretchr/testify/assert"
	"k8s-zfs/pkg"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

var loadConfigTests = []struct {
	name    string
	options controller.VolumeOptions
	result  *pkg.Config
}{
	{
		options: controller.VolumeOptions{
			PVName: "test-pv",
			Parameters: map[string]string{
				"local":            "true",
				"snapshotsEnabled": "true",
				"defaultSnapshots": " weekly,   monthly ",
			},
			PVC: &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"k8s-zfs.frq.me/snapshots": "  hourly,   frequent ",
						"k8s-zfs.frq.me/dataset":   "test-name  ",
					},
				},
			},
		},
		result: &pkg.Config{
			Local:      true,
			NFSServer:  "",
			NFSOptions: "on",
			Snapshots:  []string{"hourly", "frequent"},
			Dataset:    "test-name",
		},
	},
}

func TestLoadConfig(t *testing.T) {
	for _, testCase := range loadConfigTests {
		t.Run(testCase.name, func(tt *testing.T) {
			cfg, err := pkg.LoadConfig(testCase.options)
			assert.Nil(tt, err, "loading config should be successful")
			assert.EqualValues(tt, testCase.result, cfg, "loaded config should be equal to expected")
			fmt.Println(cfg)
		})
	}
}
