package injector

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/golang/glog"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type admitFunc func(admissionv1.AdmissionReview) *admissionv1.AdmissionResponse

// StartServer - Starts Server
func StartServer(port, urlPath string) {
	flag.Parse()

	http.HandleFunc(urlPath, handleMutation)
	server := &http.Server{
		Addr: port,
		// Validating cert from client
		// TLSConfig: configTLS(config, getClient()),
	}
	glog.Infof("Starting server at %s", server.Addr)
	var err error
	if os.Getenv("SSL_CRT_FILE_NAME") != "" && os.Getenv("SSL_KEY_FILE_NAME") != "" {
		// Starting in HTTPS mode
		err = server.ListenAndServeTLS(os.Getenv("SSL_CRT_FILE_NAME"), os.Getenv("SSL_KEY_FILE_NAME"))
	} else {
		// LOCAL DEV SERVER : Starting in HTTP mode
		err = server.ListenAndServe()
	}
	if err != nil {
		glog.Errorf("Server Start Failed : %v", err)
	}
}

func handleMutation(w http.ResponseWriter, r *http.Request) {
	var admissionReview admissionv1.AdmissionReview
	if err := json.NewDecoder(r.Body).Decode(&admissionReview); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request body: %v", err), http.StatusBadRequest)
		return
	}

	response := mutateDeployment(&admissionReview, r)
	admissionReview.Response = response

	respBytes, err := json.Marshal(admissionReview)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to marshal response: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(respBytes); err != nil {
		http.Error(w, fmt.Sprintf("failed to write response: %v", err), http.StatusInternalServerError)
	}
}

func mutateDeployment(review *admissionv1.AdmissionReview, r *http.Request) *admissionv1.AdmissionResponse {
	glog.V(2).Info("mutating deployment")
	marshalledReview, err := json.Marshal(review)
	if err != nil {
		glog.Error(err)
		return toAdmissionResponse(err)
	}
	glog.V(2).Info("review request json", string(marshalledReview))

	rawObject := review.Request.Object.Raw
	resource := &unstructured.Unstructured{}

	deserializer := codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(rawObject, nil, resource); err != nil {
		glog.Error(err)
		return toAdmissionResponse(err)
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		glog.Error(err)
		return toAdmissionResponse(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Error(err)
		return toAdmissionResponse(err)
	}

	resourceName := review.Request.Name
	resourceNamespace := review.Request.Namespace
	resourceGroupVersionKind := review.Request.Kind

	hpaList, err := clientset.AutoscalingV1().HorizontalPodAutoscalers(resourceNamespace).List(r.Context(), metav1.ListOptions{})
	if err != nil {
		glog.Error("Error fetching hpa list: ", err)
		return toAdmissionResponse(err)
	}

	for _, hpa := range hpaList.Items {
		if hpa.Spec.ScaleTargetRef.Kind == resourceGroupVersionKind.Kind &&
			hpa.Spec.ScaleTargetRef.Name == resourceName &&
			hpa.Spec.ScaleTargetRef.APIVersion == resourceGroupVersionKind.Group+"/"+resourceGroupVersionKind.Version {

			statusReplicas, found, err := unstructured.NestedInt64(resource.Object, "status", "replicas")
			if err != nil || !found {
				glog.Error("Failed to get status replicas: ", err)
				return toAdmissionResponse(fmt.Errorf("failed to get status replicas"))
			}

			patch := []byte(fmt.Sprintf("[{\"op\": \"replace\", \"path\": \"/spec/replicas\", \"value\": %d}]", statusReplicas))
			glog.V(2).Info("Generated patch: ", string(patch))

			reviewResponse := admissionv1.AdmissionResponse{}
			reviewResponse.Patch = patch
			reviewResponse.Allowed = true
			patchType := admissionv1.PatchTypeJSONPatch
			reviewResponse.PatchType = &patchType
			reviewResponse.UID = review.Request.UID
			return &reviewResponse
		}
	}

	// If no HPA is found, allow the deployment to set its own replica count
	return &admissionv1.AdmissionResponse{
		UID:     review.Request.UID,
		Allowed: true,
	}
}

func toAdmissionResponse(err error) *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{
		Result: &v1.Status{
			Message: err.Error(),
		},
		Allowed: false,
	}
}
