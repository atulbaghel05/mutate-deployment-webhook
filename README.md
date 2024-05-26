# Kubernetes Admission webhook using golang in minikube

This work is based on the project - [admission-webhook-sample](https://github.com/dinumathai/admission-webhook-sample) is a sample Kubernetes mutating admission webhook project written in golang.

### What is Dynamic Admission webhook ?

An admission webhook is an HTTPS service that is called by Kubernetes api-server when it receives a request(CREATED/UPDATED/DELETED Kubernetes resource). The webhook is called prior to persistence of the object, but after the request is authenticated and authorized. The webhook response contain the information whether to allow the Kubernetes request to proceed further. Also may contain information on changes to be done on the Kubernetes request.

There are two type of Dynamic Admission webhook -
1. Validating admission webhook
1. Mutating admission webhook

### What is Mutating admission webhook ?

Mutating admission webhooks are invoked first, and can modify objects send to the Kubernetes API server. This is usually used to inject/set some values to the Kubernetes object.

### What is Validating admission webhook ?

After all object modifications are complete and after the incoming object is validated by the API server, validating admission webhooks are invoked and can accept/reject requests. This is usually used to enforce custom policies.

## When admission webhook comes into picture ?
![admission webhook flow](./doc/persistance-flow.png)

Once the request is authenticated and authorized all the mutating admission webhook will be called, which may change the incoming object. Then the schema validation in done. And finally all the validating admission webhook are called. If all the webhooks allows the request the object is persisted to DB.

## About the project

1. In this project, we are using admission webhook for changing replica field of the deployment spec based on if hpa is configured or not. If hpa is configured, we let the exisiting replica count to be persistent and if hpa is not configured we let new deployment override the existing replica count.

## Build and deploy in minikube

To get the webhooks up and running in minikube. First we have have generate certificates for webhooks, bring up the webhook and then configure the minikube to use it. And finally test it :-).

### Prerequisites
1. Basic understanding of Kubernetes.
1. Minikube running in local machine.
1. kubectl.
1. openssl(optional)

### Create the certificate
The certificate needed for webhook is available at [deploy/ca/](deploy/ca) folder. Certificates are generated under the assumption that the namespace is `webhook` and the `service` name is `admission-webhook`. If any change in namespace or service name [deploy/ca/server.conf](deploy/ca/server.conf) must be updated and certificates needs to be regenerated. Commands to generate the all certificate files are available at [deploy/ca/README.md](deploy/ca/README.md).

### Deploy in minikube

1. Start minikube. By default `ValidatingAdmissionWebhook` and `MutatingAdmissionWebhook` will be enabled.
1. The certificates are created for the K8S service `admission-webhook` inside namespace `webhook`. If the service name or namespace is different, please re-created certificates. Refer [deploy/ca/README.md](deploy/ca/README.md)
1. Created Namespace, Deployment, Service and MutatingAdmissionWebhook objects.
```
git clone git@github.com:dinumathai/admission-webhook-sample.git
cd admission-webhook-sample
kubectl create namespace webhook
kubectl create configmap -n webhook admission-webhook-cert --from-file=deploy/ca/
docker build -t webhook:webhook-sample-image .
minikube image load webhook:webhook-sample-image
kubectl apply -f deploy/deployment.yaml
kubectl apply -f deploy/service.yaml
```
1. Makes sure that the webhook pod is up and running - `kubectl get pods -n webhook`. Once the webhook is up, create the webhook object - `kubectl apply -f deploy/webhook-admission-configuration.yaml`.

### Testing Mutation
1. Setup HPA - under directory demo-app run kubectl apply -f hpa.yaml
2. Run kubectl apply -f deployment.yaml
3. Modify the replica in deployment spec, this should be different from older replica count.
4. Again run kubectl apply -f deployment.yaml , this time new deployment should not override the older replica count.

## Reference
1. https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/
1. https://github.com/kubernetes/kubernetes/tree/release-1.9/test/images/webhook
