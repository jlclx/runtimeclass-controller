# runtimeclass-controller
A very basic admission controller that adds a runtimeClassName to an object's spec depending on namespace label.

### Example usage:

```console
jlclx@sandbox26:~/git$ kubectl create namespace runtimeclass-controller
namespace/runtimeclass-controller created
jlclx@sandbox26:~/git$ sh deployments/runtimeclass-controller.sh | kubectl apply -f -
secret/runtimeclass-controller-certs created
serviceaccount/runtimeclass-controller created
clusterrole.rbac.authorization.k8s.io/runtimeclass-controller created
clusterrolebinding.rbac.authorization.k8s.io/runtimeclass-controller created
deployment.apps/runtimeclass-controller created
service/runtimeclass-controller created
mutatingwebhookconfiguration.admissionregistration.k8s.io/runtimeclass-controller created
jlclx@sandbox26:~/git$ kubectl create namespace example
namespace/example created
jlclx@sandbox26:~/git$ kubectl label namespace example runtimeclassname-default=gvisor --overwrite
namespace/example labeled
jlclx@sandbox26:~/git$ cat <<EOF | kubectl apply --namespace example -f -
> apiVersion: v1
> kind: Pod
> metadata:
>   name: busybox
> spec:
>   containers:
>     - name: busybox-debug
>       image: busybox
>       command:
>         - sleep
>         - "3600"
>       imagePullPolicy: IfNotPresent
>   restartPolicy: Always
> EOF
pod/busybox created
jlclx@sandbox26:~/git$ kubectl get --namespace example pod busybox -o jsonpath="{.spec.runtimeClassName}" ; echo
gvisor
```

### Motivation:
Not every helm chart, executor, operator, etc supports runtimeClassName on object specs yet.
This allows me to work around that for testing different runtimes.
