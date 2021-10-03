package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	admission "k8s.io/api/admission/v1"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	log "k8s.io/klog/v2"
	"net/http"
)

type controller struct {
	deserializer runtime.Decoder
	client       *kubernetes.Clientset
}

type PatchResult struct {
	Allowed bool
	Message string
	Patches []Patch
}

type Patch struct {
	Operation string      `json:"op"`
	Path      string      `json:"path"`
	From      string      `json:"from"`
	Value     interface{} `json:"value,omitempty"`
}

type PatchIntent struct {
	runtimeClassName *string
	group            string
	namespace        string
	name             string
	path             string
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

	c := controller{
		deserializer: serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer(),
		client:       clientset,
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

func (c *controller) Mutate(w http.ResponseWriter, r *http.Request) {
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
	if _, _, err := c.deserializer.Decode(body, nil, &review); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request: %v", err), http.StatusBadRequest)
		log.Error("invalid review")
		return
	}

	if review.Request == nil {
		http.Error(w, "bad admission review", http.StatusBadRequest)
		log.Error("bad admission review")
		return
	}

	result, err := c.GetPatches(review.Request)
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

func (c *controller) GetPatches(r *admission.AdmissionRequest) (*PatchResult, error) {
	var p *PatchIntent
	log.Infof("%s triggered", r.RequestResource.Resource)
	switch r.Resource.Group {
	case "pods":
		var pod core.Pod
		if err := json.Unmarshal(r.Object.Raw, &pod); err != nil {
			return &PatchResult{
				Message: err.Error(),
			}, err
		}

		p = &PatchIntent{
			runtimeClassName: pod.Spec.RuntimeClassName,
			group:            r.Resource.Group,
			namespace:        pod.Namespace,
			name:             pod.Name,
			path:             "/spec/runtimeClassName",
		}
	}

	if p != nil {
		patches, err := c.CreatePatches(p)
		if err == nil {
			return &PatchResult{
				Allowed: true,
				Patches: *patches,
			}, nil
		}
	}

	return &PatchResult{
		Allowed: true,
	}, nil
}

func (c *controller) CreatePatches(p *PatchIntent) (*[]Patch, error) {
	var patches []Patch

	namespaceObj, err := c.client.CoreV1().Namespaces().Get(context.TODO(), p.namespace, meta.GetOptions{})
	if err != nil {
		// Currently, silently fail.
		return &patches, err
	}

	if classname, ok := namespaceObj.Labels["runtimeclassname-default"]; ok {
		if p.runtimeClassName == nil {
			log.Infof("%s/%s in %s lacks runtimeClassName, default is %s", p.namespace, p.group, p.name, classname)

			patches = append(patches, Patch{
				Operation: "add",
				Path:      p.path,
				Value:     classname,
			})

			return &patches, nil
		}
	}

	return &patches, errors.New("no patch applied")
}

func (c *controller) health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
