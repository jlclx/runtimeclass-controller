package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	admission "k8s.io/api/admission/v1"
	apps "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	log "k8s.io/klog/v2"
	"net/http"
)

type Controller struct {
	Deserializer runtime.Decoder
	Client       *kubernetes.Clientset
}

type ReviewResult struct {
	Allowed bool
	Message string
	Patches []Patch
}

type Patch struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	From  string      `json:"from"`
	Value interface{} `json:"value,omitempty"`
}

type PatchScopeData struct {
	RuntimeClassName *string
	Namespace        string
	Name             string
	PatchPath        string
}

func main() {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	c := Controller{
		Deserializer: serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer(),
		Client:       clientset,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", c.health)
	mux.HandleFunc("/mutate", c.Mutate)
	server := &http.Server{
		Addr:    fmt.Sprintf(":8443"),
		Handler: mux,
	}

	if err := server.ListenAndServeTLS("/certs/tls.crt", "/certs/tls.key"); err != nil {
		log.Errorf("failed to listen and serve: %v", err)
	}
}

func (c *Controller) Mutate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, fmt.Sprint("POST only"), http.StatusMethodNotAllowed)
		log.Error("invalid method")
		return
	}

	if contentType := r.Header.Get("Content-Type"); contentType != "application/json" {
		http.Error(w, fmt.Sprint("application/json content only"), http.StatusBadRequest)
		log.Error("invalid content/type")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("bad request body"), http.StatusBadRequest)
		log.Error("invalid body")
		return
	}

	var review admission.AdmissionReview
	if _, _, err := c.Deserializer.Decode(body, nil, &review); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request: %v", err), http.StatusBadRequest)
		log.Error("invalid review")
		return
	}

	if review.Request == nil {
		http.Error(w, "bad admission review, no request", http.StatusBadRequest)
		log.Error("bad admission review, no request")
		return
	}

	result, err := c.Review(review.Request)
	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	response := admission.AdmissionReview{
		TypeMeta: meta.TypeMeta{APIVersion: "admission.k8s.io/v1", Kind: "AdmissionReview"},
		Response: &admission.AdmissionResponse{
			UID:     review.Request.UID,
			Allowed: result.Allowed,
			Result:  &meta.Status{Message: result.Message},
		},
	}

	if len(result.Patches) > 0 {
		JSONPatch := admission.PatchTypeJSONPatch
		patches, err := json.Marshal(result.Patches)
		if err != nil {
			log.Error(err)
			http.Error(w, fmt.Sprintf("could not serialize JSON patch: %v", err), http.StatusInternalServerError)
		}
		response.Response.Patch = patches
		response.Response.PatchType = &JSONPatch
	}

	responseJson, err := json.Marshal(response)
	if err != nil {
		log.Error(err)
		http.Error(w, fmt.Sprintf("could not serialize admission response: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(responseJson)
}

func (c *Controller) Review(r *admission.AdmissionRequest) (*ReviewResult, error) {
	var patches []Patch

	resourceName := r.RequestResource.Resource

	scopeData, err := c.GetPatchScopeData(resourceName, r.Object.Raw)
	if err != nil {
		return &ReviewResult{
			Message: err.Error(),
		}, err
	}

	if scopeData != nil {
		namespaceObj, err := c.Client.CoreV1().Namespaces().Get(context.TODO(), scopeData.Namespace, meta.GetOptions{})
		if err != nil {
			return &ReviewResult{
				Message: err.Error(),
			}, err
		}

		if className, ok := namespaceObj.Labels["runtimeclassname-default"]; ok {
			if scopeData.RuntimeClassName == nil {
				log.Infof("'%s/%s' in '%s' lacks runtimeClassName, default is '%s', patching", scopeData.Namespace, scopeData.Name, resourceName, className)

				patches = append(patches, Patch{
					Op:    "add",
					Path:  scopeData.PatchPath,
					Value: className,
				})
			}
		}
	}

	return &ReviewResult{
		Allowed: true,
		Patches: patches,
	}, nil
}

func (c *Controller) GetPatchScopeData(resource string, object []byte) (*PatchScopeData, error) {
	var scopeData *PatchScopeData

	switch resource {
	case "pods":
		var pod core.Pod
		if err := json.Unmarshal(object, &pod); err != nil {
			return scopeData, err
		}

		scopeData = &PatchScopeData{
			RuntimeClassName: pod.Spec.RuntimeClassName,
			Namespace:        pod.Namespace,
			Name:             pod.Name,
			PatchPath:        "/spec/runtimeClassName",
		}
	case "deployments":
		var deployment apps.Deployment
		if err := json.Unmarshal(object, &deployment); err != nil {
			return scopeData, err
		}

		scopeData = &PatchScopeData{
			RuntimeClassName: deployment.Spec.Template.Spec.RuntimeClassName,
			Namespace:        deployment.Namespace,
			Name:             deployment.Name,
			PatchPath:        "/spec/template/spec/runtimeClassName",
		}
	case "replicasets":
		var replicaSet apps.ReplicaSet
		if err := json.Unmarshal(object, &replicaSet); err != nil {
			return scopeData, err
		}

		scopeData = &PatchScopeData{
			RuntimeClassName: replicaSet.Spec.Template.Spec.RuntimeClassName,
			Namespace:        replicaSet.Namespace,
			Name:             replicaSet.Name,
			PatchPath:        "/spec/template/spec/runtimeClassName",
		}
	case "statefulsets":
		var statefulSet apps.StatefulSet
		if err := json.Unmarshal(object, &statefulSet); err != nil {
			return scopeData, err
		}

		scopeData = &PatchScopeData{
			RuntimeClassName: statefulSet.Spec.Template.Spec.RuntimeClassName,
			Namespace:        statefulSet.Namespace,
			Name:             statefulSet.Name,
			PatchPath:        "/spec/template/spec/runtimeClassName",
		}
	case "daemonsets":
		var daemonSet apps.DaemonSet
		if err := json.Unmarshal(object, &daemonSet); err != nil {
			return scopeData, err
		}

		scopeData = &PatchScopeData{
			RuntimeClassName: daemonSet.Spec.Template.Spec.RuntimeClassName,
			Namespace:        daemonSet.Namespace,
			Name:             daemonSet.Name,
			PatchPath:        "/spec/template/spec/runtimeClassName",
		}
	case "jobs":
		var job batch.Job
		if err := json.Unmarshal(object, &job); err != nil {
			return scopeData, err
		}

		scopeData = &PatchScopeData{
			RuntimeClassName: job.Spec.Template.Spec.RuntimeClassName,
			Namespace:        job.Namespace,
			Name:             job.Name,
			PatchPath:        "/spec/template/spec/runtimeClassName",
		}
	case "cronjobs":
		var cronJob batch.CronJob
		if err := json.Unmarshal(object, &cronJob); err != nil {
			return scopeData, err
		}

		scopeData = &PatchScopeData{
			RuntimeClassName: cronJob.Spec.JobTemplate.Spec.Template.Spec.RuntimeClassName,
			Namespace:        cronJob.Namespace,
			Name:             cronJob.Name,
			PatchPath:        "/jobTemplate/spec/template/spec/runtimeClassName",
		}
	}

	return scopeData, nil
}

func (c *Controller) health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
