package platform

import (
	"context"
	"github.com/ghodss/yaml"
	"github.com/sirupsen/logrus"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kjson "k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PlatformClient - Платформенный клиент
type PlatformClient struct {
	Client          client.Client
	apiextClientset *apiextensions.Clientset
	CoreClientset   *kubernetes.Clientset
}

func NewPlatformClient(kubeConfig *rest.Config, client client.Client) *PlatformClient {
	return &PlatformClient{
		Client:          client,
		apiextClientset: apiextensions.NewForConfigOrDie(kubeConfig),
		CoreClientset:   kubernetes.NewForConfigOrDie(kubeConfig),
	}
}

func (c *PlatformClient) DeployCRD(ctx context.Context, crdData []byte) {
	var crd = c.LoadCRD(ctx, crdData)

	currentCrd, err := c.apiextClientset.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crd.Name, metav1.GetOptions{})
	if currentCrd == nil || currentCrd.Name == "" {
		_, err = c.apiextClientset.ApiextensionsV1().CustomResourceDefinitions().Create(ctx, crd, metav1.CreateOptions{})
		if err != nil {
			c.log(ctx).Fatalf("Ошибка при деплое CRD: %v; %v", err, crd)
		}
		c.log(ctx).Infof("Успешно создан CRD: %v", crd.Name)
	} else {
		crd.ResourceVersion = currentCrd.ResourceVersion
		_, err = c.apiextClientset.ApiextensionsV1().CustomResourceDefinitions().Update(ctx, crd, metav1.UpdateOptions{})
		if err != nil {
			c.log(ctx).Fatalf("Ошибка при деплое CRD: %v; %v", err, crd)
		}
		c.log(ctx).Infof("Успешно обновлен CRD: %v", crd.Name)
	}
}

func (c *PlatformClient) LoadCRD(ctx context.Context, crdData []byte) *apiextensionsv1.CustomResourceDefinition {
	var crd = apiextensionsv1.CustomResourceDefinition{}

	var jsonData, err = yaml.YAMLToJSON(crdData)
	if err != nil {
		c.log(ctx).Fatalf("Ошибка при конвертации CRD из YAML в Json: %v; %v", err, string(crdData))
	}

	err = kjson.Unmarshal(jsonData, &crd)
	if err != nil {
		c.log(ctx).Fatalf("Ошибка при десериализации CRD: %v; %v", err, string(crdData))
	}

	return &crd
}

func (c *PlatformClient) log(ctx context.Context) *logrus.Entry {
	return ctx.Value("log").(*logrus.Entry)
}
