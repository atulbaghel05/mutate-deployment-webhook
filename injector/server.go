package injector

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/golang/glog"
	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	deploymentResource := metav1.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	if review.Request.Resource != deploymentResource {
		glog.Errorf("expect resource to be %s", &deploymentResource)
		return nil
	}

	rawObject := review.Request.Object.Raw
	deployment := appsv1.Deployment{}

	deserializer := codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(rawObject, nil, &deployment); err != nil {
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

	hpa, err := clientset.AutoscalingV1().HorizontalPodAutoscalers(deployment.Namespace).Get(r.Context(), deployment.Name, metav1.GetOptions{})
	if err != nil {
		glog.Error("Error fetching hpa object ", err)
		return toAdmissionResponse(err)
	}

	if hpa != nil {
		statusReplicas := deployment.Status.Replicas
		patch := []byte(fmt.Sprintf("[{\"op\": \"replace\", \"path\": \"/spec/replicas\", \"value\": %d}]", statusReplicas))

		glog.V(2).Info("Generated patch: %s\n", string(patch))

		reviewResponse := admissionv1.AdmissionResponse{}
		reviewResponse.Patch = patch
		reviewResponse.Allowed = true
		patchType := admissionv1.PatchTypeJSONPatch
		reviewResponse.PatchType = &patchType
		reviewResponse.UID = review.Request.UID
		return &reviewResponse
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
