package main

import (
	"flag"
	"go.uber.org/zap"
	"k8s-zfs/pkg"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"syscall"

	"github.com/kubernetes-sigs/sig-storage-lib-external-provisioner/controller"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

func main() {
	syscall.Umask(0)

	flag.Parse()
	flag.Set("logtostderr", "true")

	// todo make it configurable and use Prod by default
	l, err := zap.NewDevelopment()
	if err != nil {
		panic("failed to initialize zap logger: " + err.Error())
	}
	// todo defer rollback
	zap.ReplaceGlobals(l)
	zap.RedirectStdLog(l)

	// Look for KUBECONFIG env variable or use InClusterConfig if not set
	config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	if err != nil {
		zap.S().Fatalf("Failed to load config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		zap.S().Fatalf("Failed to create client: %v", err)
	}

	serverVersion, err := clientset.Discovery().ServerVersion()
	if err != nil {
		zap.S().Fatalf("Error getting server version: %v", err)
	}

	provisioner, err := pkg.NewZfsProvisioner(clientset)
	if err != nil {
		zap.S().Fatalf("error creating zfs provisioner: %v", err)
	}

	pc := controller.NewProvisionController(clientset, pkg.Namespace, provisioner, serverVersion.GitVersion)
	pc.Run(wait.NeverStop)
}
