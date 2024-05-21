package controllers

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"k8s.io/api/core/v1"
	"k8s.io/api/rbac/v1beta1"

	"github.com/sirupsen/logrus"
	platformv1 "github.com/wbe7/dynamicnamespace/api/v1"
	"github.com/wbe7/dynamicnamespace/config/crd"
	"github.com/wbe7/dynamicnamespace/internal/platform"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	defaultFinalizer = platformv1.GroupVersion.Group + "/finalizer"
	defaultLabelKey  = platformv1.GroupVersion.Group + "/created-by"
)

// DynamicNamespaceReconciler reconciles a DynamicNamespace object
type DynamicNamespaceReconciler struct {
	client.Client
	*platform.PlatformClient
	log    *logrus.Entry
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=platform.cloudnative.space,resources=dynamicnamespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;create;update
// +kubebuilder:rbac:groups=platform.cloudnative.space,resources=dynamicnamespaces/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=platform.cloudnative.space,resources=dynamicnamespaces/finalizers,verbs=update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=resourcequotas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=namespaces/status,verbs=get;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DynamicNamespace object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *DynamicNamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var log = r.log.WithField("dynamicnamespace", req.NamespacedName)
	log.Infof("> Начало обработки ресурса: %v", req.NamespacedName)

	// Получение данных ресурса из k8s
	var desiredResource platformv1.DynamicNamespace
	var err = r.Get(ctx, req.NamespacedName, &desiredResource)
	if err != nil {
		if kerrors.IsNotFound(err) {
			log.Debug("Ресурс был удален ранее")
			return ctrl.Result{}, nil
		}
		log.Errorf("< Ошибка при чтении CR DynamicNamespace: %v", err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Проверка удаления
	var deleted = desiredResource.GetDeletionTimestamp() != nil
	if deleted {
		err = r.processDefaultFinalization(log, ctx, &desiredResource, r.finalize)
		if err != nil {
			log.Errorf("Ошибка при финализации ресурса %v: %v", desiredResource.GetName(), err)
			r.updateStatus(log, ctx, &desiredResource, &platformv1.DynamicNamespaceStatus{Code: "ERROR", Message: err.Error()})
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		return ctrl.Result{}, nil
	}

	// Добавление финализатора в ресурс
	if !r.hasDefaultFinalizer(&desiredResource) {
		err = r.InjectDefaultFinalizer(ctx, &desiredResource)
		if err != nil {
			log.Errorf("Ошибка при добавлении финализатора в ресурс %v: %v", desiredResource.GetName(), err)
			r.updateStatus(log, ctx, &desiredResource, &platformv1.DynamicNamespaceStatus{Code: "ERROR", Message: err.Error()})
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		return ctrl.Result{}, nil
	}

	// Прикладная валидация ресурса
	err = r.validate(&desiredResource)
	if err != nil {
		log.Errorf("Ошибка при валидации ресурса %v: %v", desiredResource.GetName(), err)
		r.updateStatus(log, ctx, &desiredResource, &platformv1.DynamicNamespaceStatus{Code: "ERROR", Message: err.Error()})
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	err = r.createOrUpdateNamespace(ctx, &desiredResource)
	if err != nil {
		log.Errorf("Ошибка при создании ресурса %v: %v", desiredResource.GetName(), err)
		r.updateStatus(log, ctx, &desiredResource, &platformv1.DynamicNamespaceStatus{Code: "ERROR", Message: err.Error()})
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	err = r.createOrUpdateResourceQuota(ctx, &desiredResource)
	if err != nil {
		log.Errorf("Ошибка при создании ресурса %v: %v", desiredResource.GetName(), err)
		r.updateStatus(log, ctx, &desiredResource, &platformv1.DynamicNamespaceStatus{Code: "ERROR", Message: err.Error()})
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	err = r.createOrUpdateRoleBinding(ctx, &desiredResource)
	if err != nil {
		log.Errorf("Ошибка при создании ресурса %v: %v", desiredResource.GetName(), err)
		r.updateStatus(log, ctx, &desiredResource, &platformv1.DynamicNamespaceStatus{Code: "ERROR", Message: err.Error()})
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	r.updateStatus(log, ctx, &desiredResource, &platformv1.DynamicNamespaceStatus{Code: "ACTIVE", Message: "Все хорошо"})

	// Выход из цикла
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DynamicNamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.log = logrus.WithField("controller", "dynamicnamespace")
	r.PlatformClient = platform.NewPlatformClient(mgr.GetConfig(), r.Client)

	var ctx = context.WithValue(context.Background(), "log", r.log)

	// Создание или обновление CRD ресурса
	r.DeployCRD(ctx, crd.DynamicNamespace)
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1.DynamicNamespace{}).
		Complete(r)
}

func (r *DynamicNamespaceReconciler) updateStatus(log *logrus.Entry, ctx context.Context, resource *platformv1.DynamicNamespace, status *platformv1.DynamicNamespaceStatus) {
	if !reflect.DeepEqual(status, resource.Status) {
		resource.Status = *status
		var err = r.Client.Status().Update(ctx, resource)
		if err != nil {
			log.Errorf("  Ошибка при обновлении статуса ресурса [%v.%v]: %v", resource.GetName(), resource.GetNamespace(), err)
		}
	}
}

func (r *DynamicNamespaceReconciler) hasDefaultFinalizer(resource *platformv1.DynamicNamespace) bool {
	return controllerutil.ContainsFinalizer(resource, defaultFinalizer)
}

func (r *DynamicNamespaceReconciler) InjectDefaultFinalizer(ctx context.Context, resource *platformv1.DynamicNamespace) error {
	controllerutil.AddFinalizer(resource, defaultFinalizer)
	return r.Client.Update(ctx, resource)
}

func (r *DynamicNamespaceReconciler) processDefaultFinalization(
	log *logrus.Entry,
	ctx context.Context,
	resource *platformv1.DynamicNamespace,
	finalizer func(resource *platformv1.DynamicNamespace) error,
) error {
	if controllerutil.ContainsFinalizer(resource, defaultFinalizer) {
		// Запуск логики финализации ресурса
		if err := finalizer(resource); err != nil {
			return err
		}

		// Удаление финалайзера и ресурса
		controllerutil.RemoveFinalizer(resource, defaultFinalizer)
		err := r.Client.Update(ctx, resource)
		if err != nil {
			return err
		}
		log.Infof("Успешно удалён финалайзер ресурса [%v.%v]", resource.GetName(), resource.GetNamespace())
		log.Infof("Успешно удалён ресурс [%v.%v]", resource.GetName(), resource.GetNamespace())
	} else {
		log.Infof("Успешно удалён ресурс [%v.%v]", resource.GetName(), resource.GetNamespace())
	}

	return nil
}

func (r *DynamicNamespaceReconciler) finalize(resource *platformv1.DynamicNamespace) error {
	r.log.Infof("Финализация ресурса: %v", resource.Name)
	//Проверка на основании лейбла или аннотации
	//Если есть нужная метка, подтверждающая, что этот ресурс наш, то удаляем
	// TODO: реализовать проверку метки

	desiredNamespace, err := generateNamespace(resource)
	if err != nil {
		return err
	}

	//Проверка есть ли у созданного ns нужный label
	namespace := &v1.Namespace{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: resource.Name}, namespace)
	if err != nil && kerrors.IsNotFound(err) {
		r.log.Infof("Целевой ресурс Application [%v.%v] уже удален", resource.GetName(), resource.GetNamespace())
		return nil
	}
	namespaceLabels := namespace.GetLabels()

	if namespaceLabels[defaultLabelKey] == fmt.Sprintf("%s.%s", resource.Namespace, resource.Name) {
		err = r.Delete(context.TODO(), desiredNamespace)
		if err != nil {
			return err
		}
		r.log.Infof("Успешно удален целевой Namespace [%v.%v]", resource.GetName(), resource.GetNamespace())
	} else {
		r.log.Infof("Целевой Namespace [%v.%v] не содержит нужного лейбла", resource.GetName(), resource.GetNamespace())
	}

	return nil
}

func (r *DynamicNamespaceReconciler) validate(resource *platformv1.DynamicNamespace) error {
	r.log.Infof("Валидация ресурса: %v", resource.Name)

	//Проверка есть ли у созданного ns нужный label
	namespace := &v1.Namespace{}
	err := r.Get(context.TODO(), types.NamespacedName{Name: resource.Name}, namespace)
	if err != nil && kerrors.IsNotFound(err) {
		return nil
	} else {
		namespaceLabels := namespace.GetLabels()
		// Если лейбл есть, то ресурс обновляется
		if namespaceLabels[defaultLabelKey] == fmt.Sprintf("%s.%s", resource.Namespace, resource.Name) {
			return nil
		}
		return errors.New("namespace с таким именем уже существует")
	}
}

func (r *DynamicNamespaceReconciler) createOrUpdateNamespace(ctx context.Context, resource *platformv1.DynamicNamespace) error {
	r.log.Infof("Прикладная логика над ресурсом: %v", resource.Name)
	desiredNamespace, err := generateNamespace(resource)
	if err != nil {
		return err
	}

	namespace := &v1.Namespace{}

	err = r.Get(ctx, types.NamespacedName{Name: resource.Name}, namespace)

	if err != nil && kerrors.IsNotFound(err) {
		r.log.Infof("Целевой Namespace [%v] не создан, создаю...", desiredNamespace.GetName())
		err = r.Create(ctx, desiredNamespace)
		if err != nil {
			return err
		}
		r.log.Infof("Целевой Namespace [%v] успешно создан", desiredNamespace.GetName())
		//Создаем Quota и SA
		//TODO: Create SA
		//TODO: Create secret with SA token in initial namespace
		//TODO: Create default ResourceQuota
	} else {
		//Если NS уже создан (что делаем?)
		//Проверка на базе версии ресурса?
		//Проверка на основании лейбла или аннотации
		//Создаем/удаляем SA или Quota
		//TODO: реализовать создание удаление SA/Quota при обновлении DN, если уже создан NS
	}
	return nil
}

func (r *DynamicNamespaceReconciler) createOrUpdateResourceQuota(ctx context.Context, resource *platformv1.DynamicNamespace) error {
	r.log.Infof("Создаем квоту для неймспейса: %v", resource.Name)
	desiredResourceQuota, err := generateResourceQuota(resource)
	if err != nil {
		return err
	}

	quota := &v1.ResourceQuota{}
	err = r.Get(ctx, types.NamespacedName{
		Namespace: desiredResourceQuota.GetNamespace(),
		Name:      desiredResourceQuota.GetName(),
	}, quota)

	if err != nil && kerrors.IsNotFound(err) {
		r.log.Infof("Целевая ResourceQuota [%v] не создана, создаю...", desiredResourceQuota.GetName())
		err = r.Create(ctx, desiredResourceQuota)
		if err != nil {
			return err
		}
		r.log.Infof("Целевая ResourceQuota [%v] успешно создана", desiredResourceQuota.GetName())
	}
	return nil
}

func (r *DynamicNamespaceReconciler) createOrUpdateRoleBinding(ctx context.Context, resource *platformv1.DynamicNamespace) error {
	r.log.Infof("Создаем RoleBinding для неймспейса: %v", resource.Name)
	desiredRoleBinding, err := generateRoleBinding(resource)
	if err != nil {
		return err
	}

	quota := &v1beta1.RoleBinding{}
	err = r.Get(ctx, types.NamespacedName{
		Namespace: desiredRoleBinding.GetNamespace(),
		Name:      desiredRoleBinding.GetName(),
	}, quota)

	if err != nil && kerrors.IsNotFound(err) {
		r.log.Infof("Целевая RoleBinding [%v] не создана, создаю...", desiredRoleBinding.GetName())
		err = r.Create(ctx, desiredRoleBinding)
		if err != nil {
			return err
		}
		r.log.Infof("Целевая RoleBinding [%v] успешно создана", desiredRoleBinding.GetName())
	}
	return nil
}

func generateNamespace(resource *platformv1.DynamicNamespace) (*v1.Namespace, error) {
	labels := map[string]string{
		defaultLabelKey: fmt.Sprintf("%s.%s", resource.Namespace, resource.Name),
	}
	return &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   resource.Name,
			Labels: labels,
		},
	}, nil
}

func generateResourceQuota(resource *platformv1.DynamicNamespace) (*v1.ResourceQuota, error) {
	return &v1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-resourcequota", resource.Name),
			Namespace: resource.Name,
		},
		Spec: v1.ResourceQuotaSpec{
			Hard: resource.Spec.CreateQuota,
		},
	}, nil
}

func generateRoleBinding(resource *platformv1.DynamicNamespace) (*v1beta1.RoleBinding, error) {
	return &v1beta1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-rolebinding", resource.Name),
			Namespace: resource.Name,
		},
		RoleRef: v1beta1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "admin",
		},
		Subjects: resource.Spec.RoleBindingSubjects,
	}, nil
}
