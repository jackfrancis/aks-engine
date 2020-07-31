// This file is safe to edit. Once it exists it will not be overwritten

package restapi

import (
	"crypto/tls"
	"net/http"

	"github.com/Azure/aks-engine/cmd"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/middleware"

	"github.com/Azure/aks-engine/pkg/swagger/models"
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
		if to.String(params.Body.AzureEnvironment) == "" {
			params.Body.AzureEnvironment = to.StringPtr(cmd.DefaultAzureEnvironment)
		}
		if to.String(params.Body.Location) == "" {
			params.Body.Location = to.StringPtr(cmd.DefaultLocation)
		}
		if to.String(params.Body.ControlPlaneVMType) == "" {
			params.Body.ControlPlaneVMType = to.StringPtr(cmd.DefaultControlPlaneVMType)
		}
		if to.String(params.Body.NodeVMType) == "" {
			params.Body.NodeVMType = to.StringPtr(cmd.DefaultNodeVMType)
		}
		if to.String(params.Body.KubernetesVersion) == "" {
			params.Body.KubernetesVersion = to.StringPtr(cmd.DefaultKubernetesVersion)
		}
		if params.Body.ControlPlaneNodes == 0 {
			params.Body.ControlPlaneNodes = int64(cmd.DefaultControlPlaneNodes)
		}
		if params.Body.Nodes == 0 {
			params.Body.Nodes = int64(cmd.DefaultNodes)
		}

		cc := cmd.CreateCmd{
			MgmtClusterKubeConfigPath: to.String(params.Body.MgmtClusterKubeConfigPath),
			SubscriptionID:            to.String(params.Body.SubscriptionID),
			TenantID:                  to.String(params.Body.TenantID),
			ClientID:                  to.String(params.Body.ClientID),
			ClientSecret:              to.String(params.Body.ClientSecret),
			AzureEnvironment:          to.String(params.Body.AzureEnvironment),
			ClusterName:               to.String(params.Body.ClusterName),
			VnetName:                  to.String(params.Body.VnetName),
			ResourceGroup:             to.String(params.Body.ResourceGroup),
			Location:                  to.String(params.Body.Location),
			ControlPlaneVMType:        to.String(params.Body.ControlPlaneVMType),
			NodeVMType:                to.String(params.Body.NodeVMType),
			SSHPublicKey:              to.String(params.Body.SSHPublicKey),
			KubernetesVersion:         "1.17.8",
			ControlPlaneNodes:         int(params.Body.ControlPlaneNodes),
			Nodes:                     int(params.Body.Nodes),
		}
		err := cc.Run()
		if err != nil {
			return operations.NewCreateClusterOK().WithPayload(&models.CreateData{
				ClusterName: to.StringPtr(cc.ClusterName),
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
