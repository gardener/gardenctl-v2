module github.com/gardener/gardenctl-v2

go 1.16

require (
	github.com/Masterminds/sprig/v3 v3.2.2
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/gardener/gardener v1.25.0
	github.com/gardener/gardener-extension-provider-openstack v1.20.0
	github.com/golang/mock v1.6.0
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/googleapis/gnostic v0.5.5 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.10.5
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.8.1
	golang.org/x/crypto v0.0.0-20210220033148-5ea612d1eb83
	golang.org/x/oauth2 v0.0.0-20210514164344-f6687ab2804c // indirect
	golang.org/x/sys v0.0.0-20211004093028-2c5d950f24ef // indirect
	golang.org/x/tools v0.1.7 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	k8s.io/api v0.20.8
	k8s.io/apimachinery v0.20.8
	k8s.io/cli-runtime v0.20.8
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/component-base v0.20.8
	k8s.io/klog/v2 v2.8.0
	k8s.io/kube-openapi v0.0.0-20210421082810-95288971da7e // indirect
	k8s.io/utils v0.0.0-20210517184530-5a248b5acedc
	sigs.k8s.io/controller-runtime v0.8.3
	sigs.k8s.io/structured-merge-diff/v4 v4.1.1 // indirect
)

replace (
	k8s.io/client-go => k8s.io/client-go v0.20.8
	k8s.io/code-generator => k8s.io/code-generator v0.20.8
)
