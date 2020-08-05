// This file is safe to edit. Once it exists it will not be overwritten

package restapi

import (
	"crypto/tls"
	"net/http"

	"github.com/Azure/aks-engine/pkg/swagger/models"
	"github.com/Azure/aks-engine/pkg/v2/engine"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/middleware"

	"github.com/Azure/aks-engine/pkg/swagger/restapi/operations"
)

//go:generate swagger generate server --target ../../swagger --name Aksengine --spec ../../api/swagger-spec/swagger.yml --principal interface{}

func configureFlags(api *operations.AksengineAPI) {
	// api.CommandLineOptionsGroups = []swag.CommandLineOptionsGroup{ ... }
}

func configureAPI(api *operations.AksengineAPI) http.Handler {
	// configure the api here
	api.ServeError = errors.ServeError

	// Set your custom logger if needed. Default one is log.Printf
	// Expected interface func(string, ...interface{})
	//
	// Example:
	// api.Logger = log.Printf

	api.UseSwaggerUI()
	// To continue using redoc as your UI, uncomment the following line
	// api.UseRedoc()

	api.JSONConsumer = runtime.JSONConsumer()

	api.JSONProducer = runtime.JSONProducer()

	api.CreateClusterHandler = operations.CreateClusterHandlerFunc(func(params operations.CreateClusterParams) middleware.Responder {
		clusterSpec := models.CreateData{
			MgmtClusterKubeConfig: params.Body.MgmtClusterKubeConfig,
			SubscriptionID:        params.Body.SubscriptionID,
			TenantID:              params.Body.TenantID,
			ClientID:              params.Body.ClientID,
			ClientSecret:          params.Body.ClientSecret,
			AzureEnvironment:      params.Body.AzureEnvironment,
			ClusterName:           params.Body.ClusterName,
			VnetName:              params.Body.VnetName,
			ResourceGroup:         params.Body.ResourceGroup,
			Location:              params.Body.Location,
			ControlPlaneVMType:    params.Body.ControlPlaneVMType,
			NodeVMType:            params.Body.NodeVMType,
			SSHPublicKey:          params.Body.SSHPublicKey,
			KubernetesVersion:     params.Body.KubernetesVersion,
			ControlPlaneNodes:     params.Body.ControlPlaneNodes,
			Nodes:                 params.Body.Nodes,
		}
		if to.String(clusterSpec.AzureEnvironment) == "" {
			clusterSpec.AzureEnvironment = to.StringPtr(engine.DefaultAzureEnvironment)
		}
		if to.String(clusterSpec.Location) == "" {
			clusterSpec.Location = to.StringPtr(engine.DefaultLocation)
		}
		if to.String(clusterSpec.ControlPlaneVMType) == "" {
			clusterSpec.ControlPlaneVMType = to.StringPtr(engine.DefaultControlPlaneVMType)
		}
		if to.String(clusterSpec.NodeVMType) == "" {
			clusterSpec.NodeVMType = to.StringPtr(engine.DefaultNodeVMType)
		}
		if to.String(clusterSpec.KubernetesVersion) == "" {
			clusterSpec.KubernetesVersion = to.StringPtr(engine.DefaultKubernetesVersion)
		}
		if clusterSpec.ControlPlaneNodes == 0 {
			clusterSpec.ControlPlaneNodes = int64(engine.DefaultControlPlaneNodes)
		}
		if clusterSpec.Nodes == 0 {
			clusterSpec.Nodes = int64(engine.DefaultNodes)
		}

		cluster := engine.NewCluster(clusterSpec)
		err := cluster.Create()
		if err != nil {
			return operations.NewCreateClusterOK().WithPayload(&models.CreateData{
				ClusterName: to.StringPtr(to.String(clusterSpec.ClusterName)),
			})
		}
		return operations.NewCreateClusterDefault(http.StatusInternalServerError)
	})
	if api.HealthzHandler == nil {
		api.HealthzHandler = operations.HealthzHandlerFunc(func(params operations.HealthzParams) middleware.Responder {
			return middleware.NotImplemented("operation operations.Healthz has not yet been implemented")
		})
	}

	api.PreServerShutdown = func() {}

	api.ServerShutdown = func() {}

	return setupGlobalMiddleware(api.Serve(setupMiddlewares))
}

// The TLS configuration before HTTPS server starts.
func configureTLS(tlsConfig *tls.Config) {
	// Make all necessary changes to the TLS configuration here.
}

// As soon as server is initialized but not run yet, this function will be called.
// If you need to modify a config, store server instance to stop it individually later, this is the place.
// This function can be called multiple times, depending on the number of serving schemes.
// scheme value will be set accordingly: "http", "https" or "unix"
func configureServer(s *http.Server, scheme, addr string) {
}

// The middleware configuration is for the handler executors. These do not apply to the swagger.json document.
// The middleware executes after routing but before authentication, binding and validation
func setupMiddlewares(handler http.Handler) http.Handler {
	return handler
}

// The middleware configuration happens before anything, this middleware also applies to serving the swagger.json document.
// So this is a good place to plug in a panic handling middleware, logging and metrics
func setupGlobalMiddleware(handler http.Handler) http.Handler {
	return handler
}
