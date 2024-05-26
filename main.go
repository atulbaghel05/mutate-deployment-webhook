package main

import "github.com/dinumathai/admission-webhook-sample/injector"

func main() {
	injector.StartServer(":8080", "/mutate-replica-count")
}
