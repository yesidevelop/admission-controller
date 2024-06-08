package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"flag"

	"github.com/gorilla/mux"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	annotationKey   = "my-annotation"
	annotationValue = "added-by-webhook"
	port            = 8443
)

func main() {

	var tlsKey, tlsCert string
	flag.StringVar(&tlsKey, "tlsKey", "/etc/certs/tls.key", "TLS key")
	flag.StringVar(&tlsCert, "tlsCert", "/etc/certs/tls.crt", "TLS certificate")
	flag.Parse()


	router := mux.NewRouter()
	router.HandleFunc("/mutate", mutate)
	fmt.Printf("Server listening on port %d\n", port)
	http.ListenAndServeTLS(":8443", tlsCert, tlsKey, router)
	
}

func mutate(w http.ResponseWriter, r *http.Request) {
	var review admissionv1.AdmissionReview

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "could not read request body", http.StatusBadRequest)
		return
	}

	if err := json.Unmarshal(body, &review); err != nil {
		http.Error(w, "could not decode request body", http.StatusBadRequest)
		return
	}

	if review.Request.Kind.Kind != "Pod" {
		http.Error(w, "expected pod object", http.StatusBadRequest)
		return
	}

	pod := corev1.Pod{}
	if err := json.Unmarshal(review.Request.Object.Raw, &pod); err != nil {
		http.Error(w, "could not decode pod object", http.StatusBadRequest)
		return
	}

	if pod.Namespace != "default" {
		// Skip pods not in the default namespace
		review.Response = &admissionv1.AdmissionResponse{
			UID:     review.Request.UID,
			Allowed: true,
		}
	} else {
		if err := addAnnotation(&pod); err != nil {
			http.Error(w, fmt.Sprintf("error adding annotation: %v", err), http.StatusInternalServerError)
			return
		}

		patchBytes, err := patchPod(pod)
		if err != nil {
			http.Error(w, fmt.Sprintf("error creating patch: %v", err), http.StatusInternalServerError)
			return
		}

		review.Response = &admissionv1.AdmissionResponse{
			UID:       review.Request.UID,
			Allowed:   true,
			Patch:     patchBytes,
			PatchType: func() *admissionv1.PatchType { pt := admissionv1.PatchTypeJSONPatch; return &pt }(),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(review)
}

func addAnnotation(pod *corev1.Pod) error {
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	pod.Annotations[annotationKey] = annotationValue
	return nil
}

func patchPod(pod corev1.Pod) ([]byte, error) {
	patch, err := json.Marshal([]map[string]interface{}{
		{
			"op":   "add",
			"path": "/metadata/annotations",
			"value": pod.Annotations,
		},
	})
	if err != nil {
		return nil, err
	}
	return patch, nil
}
