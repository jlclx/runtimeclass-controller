# runtimeclass-controller
A very basic admission controller that adds a runtimeClassName to an object's spec depending on namespace label.

### Example usage:

```console
$ sh deployments/runtimeclass-controller.sh | kubectl apply -f -
$ kubectl create namespace example
$ kubectl label namespace example runtimeclassname-default=gvisor --overwrite
$ cat <<EOF | kubectl apply --namespace example -f -
apiVersion: v1
kind: Pod
metadata:
  name: busybox
spec:
  containers:
    - name: busybox-debug
      image: busybox
      command:
        - sleep
        - "3600"
      imagePullPolicy: IfNotPresent
  restartPolicy: Always
EOF
$ kubectl get --namespace example pod busybox -o jsonpath="{.spec.runtimeClassName}" ; echo
gvisor
```