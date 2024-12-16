package main

import (
	"context"
	"fmt"
	"k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"log"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1" // 导入 corev1 包
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func main() {

	// 使用 Kubernetes Pod 内的默认服务账户连接
	config, err := rest.InClusterConfig()
	//config, err := clientcmd.BuildConfigFromFlags("", "/Users/mohaijiang/.kube/config_x2")
	if err != nil {
		log.Fatalf("Error getting in-cluster config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating Kubernetes client: %v", err)
	}

	namespace := os.Getenv("NAMESPACE")
	// 获取 Kubernetes 集群的 Secrets 客户端
	secretsClient := clientset.CoreV1().Secrets(namespace) // 监控的命名空间

	// 存储上一次 Secret 的 resourceVersion
	var lastResourceVersion string

	secretName := os.Getenv("SECRET_NAME")

	fmt.Println("SECRET_NAME:", secretName)
	// 持续监控 Secret
	err = watchSecretChange(secretsClient, secretName, &lastResourceVersion, clientset)
	if err != nil {
		log.Fatalf("Error monitoring secret: %v", err)
	}
}

func watchSecretChange(secretsClient v1.SecretInterface, secretName string, lastResourceVersion *string, clientset *kubernetes.Clientset) error {
	namespace := os.Getenv("NAMESPACE")
	deployType := os.Getenv("DEPLOY_TYPE")
	deployName := os.Getenv("DEPLOY_NAME")

	fmt.Println("namespace:", namespace)
	fmt.Println("deployType:", deployType)
	fmt.Println("deployName:", deployName)
	// 使用 Watch 监视 Secret 的变化
	watcher, err := secretsClient.Watch(context.Background(), metav1.ListOptions{
		FieldSelector: "metadata.name=" + secretName,
	})
	if err != nil {
		return fmt.Errorf("failed to watch secret: %v", err)
	}
	defer watcher.Stop()

	// 无限循环，持续监控 Secret
	for event := range watcher.ResultChan() {
		switch event.Type {
		case "MODIFIED":
			// 检查 Secret 是否更新（通过 resourceVersion）
			if secret, ok := event.Object.(*corev1.Secret); ok {
				if *lastResourceVersion != secret.ResourceVersion {
					log.Printf("Secret '%s' has been updated (resourceVersion: %s)", secretName, secret.ResourceVersion)

					// 更新记录的 resourceVersion
					*lastResourceVersion = secret.ResourceVersion
					var err error
					if deployType == "statefulset" {
						// Secret 发生变化，触发 StatefulSet 重启
						err = restartStatefulset(clientset, namespace, deployName)
					} else {
						err = restartDeployment(clientset, namespace, deployName)
					}
					if err != nil {
						log.Printf("Failed to restart StatefulSet: %v", err)
					} else {
						log.Println("Successfully triggered StatefulSet restart")
					}
				}
			}

		default:
			log.Printf("Unknown event type: %v", event.Type)
		}
	}

	return nil
}

func restartDeployment(clientset *kubernetes.Clientset, namespace string, deploymentName string) error {
	deployment, err := clientset.AppsV1().Deployments(namespace).Get(context.Background(), deploymentName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get StatefulSet: %v", err)
	}
	// 修改 Deployment 的 annotation 触发重启
	// 使用当前时间戳作为唯一值来更新 annotation
	podAnnotations := deployment.Spec.Template.Annotations
	if podAnnotations == nil {
		podAnnotations = make(map[string]string)
	}
	podAnnotations["secret-reload"] = time.Now().String()
	deployment.Spec.Template.Annotations = podAnnotations

	// 更新 Deployment，触发 Pod 重启
	_, err = clientset.AppsV1().Deployments(namespace).Update(context.Background(), deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update StatefulSet: %v", err)
	}
	return nil
}

func restartStatefulset(clientset *kubernetes.Clientset, namespace, statefulSetName string) error {
	// 获取 StatefulSet
	statefulSet, err := clientset.AppsV1().StatefulSets(namespace).Get(context.Background(), statefulSetName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get StatefulSet: %v", err)
	}

	// 修改 StatefulSet 的 annotation 触发重启
	// 使用当前时间戳作为唯一值来更新 annotation
	annotations := statefulSet.Spec.Template.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations["secret-reload"] = time.Now().String()
	statefulSet.Spec.Template.SetAnnotations(annotations)

	// 更新 StatefulSet，触发 Pod 重启
	_, err = clientset.AppsV1().StatefulSets(namespace).Update(context.Background(), statefulSet, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update StatefulSet: %v", err)
	}

	return nil
}
