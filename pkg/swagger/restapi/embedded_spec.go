// Code generated by go-swagger; DO NOT EDIT.

package restapi

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"encoding/json"
)

var (
	// SwaggerJSON embedded version of the swagger document used at generation time
	SwaggerJSON json.RawMessage
	// FlatSwaggerJSON embedded flattened version of the swagger document used at generation time
	FlatSwaggerJSON json.RawMessage
)

func init() {
	SwaggerJSON = json.RawMessage([]byte(`{
  "consumes": [
    "application/json"
  ],
  "produces": [
    "application/json"
  ],
  "schemes": [
    "http"
  ],
  "swagger": "2.0",
  "info": {
    "title": "AKS Engine",
    "version": "2.0.0-alpha"
  },
  "paths": {
    "/healthz": {
      "get": {
        "summary": "API server health check",
        "operationId": "healthz",
        "responses": {
          "200": {
            "description": "API server is healthy"
          },
          "default": {
            "description": "unexpected error"
          }
        }
      }
    },
    "/v2/create": {
      "post": {
        "summary": "create a new Kubernetes cluster",
        "operationId": "createCluster",
        "parameters": [
          {
            "name": "body",
            "in": "body",
            "required": true,
            "schema": {
              "$ref": "#/definitions/createData"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "create cluster response",
            "schema": {
              "$ref": "#/definitions/createData"
            }
          },
          "default": {
            "description": "unexpected error",
            "schema": {
              "$ref": "#/definitions/error"
            }
          }
        }
      }
    }
  },
  "definitions": {
    "createData": {
      "type": "object",
      "properties": {
        "azureEnvironment": {
          "type": "string"
        },
        "clientID": {
          "type": "string"
        },
        "clientSecret": {
          "type": "string"
        },
        "clusterName": {
          "type": "string"
        },
        "controlPlaneNodes": {
          "type": "integer"
        },
        "controlPlaneVMType": {
          "type": "string"
        },
        "kubernetesVersion": {
          "type": "string"
        },
        "location": {
          "type": "string"
        },
        "mgmtClusterKubeConfigPath": {
          "type": "string"
        },
        "nodeVMType": {
          "type": "string"
        },
        "nodes": {
          "type": "integer"
        },
        "resourceGroup": {
          "type": "string"
        },
        "sshPublicKey": {
          "type": "string"
        },
        "subscriptionID": {
          "type": "string"
        },
        "tenantID": {
          "type": "string"
        },
        "vnetName": {
          "type": "string"
        }
      }
    },
    "error": {
      "type": "object",
      "required": [
        "code",
        "message"
      ],
      "properties": {
        "code": {
          "type": "integer",
          "format": "int64"
        },
        "message": {
          "type": "string"
        }
      }
    }
  },
  "securityDefinitions": {
    "basic": {
      "type": "basic"
    }
  }
}`))
	FlatSwaggerJSON = json.RawMessage([]byte(`{
  "consumes": [
    "application/json"
  ],
  "produces": [
    "application/json"
  ],
  "schemes": [
    "http"
  ],
  "swagger": "2.0",
  "info": {
    "title": "AKS Engine",
    "version": "2.0.0-alpha"
  },
  "paths": {
    "/healthz": {
      "get": {
        "summary": "API server health check",
        "operationId": "healthz",
        "responses": {
          "200": {
            "description": "API server is healthy"
          },
          "default": {
            "description": "unexpected error"
          }
        }
      }
    },
    "/v2/create": {
      "post": {
        "summary": "create a new Kubernetes cluster",
        "operationId": "createCluster",
        "parameters": [
          {
            "name": "body",
            "in": "body",
            "required": true,
            "schema": {
              "$ref": "#/definitions/createData"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "create cluster response",
            "schema": {
              "$ref": "#/definitions/createData"
            }
          },
          "default": {
            "description": "unexpected error",
            "schema": {
              "$ref": "#/definitions/error"
            }
          }
        }
      }
    }
  },
  "definitions": {
    "createData": {
      "type": "object",
      "properties": {
        "azureEnvironment": {
          "type": "string",
          "minLength": 0
        },
        "clientID": {
          "type": "string",
          "minLength": 0
        },
        "clientSecret": {
          "type": "string",
          "minLength": 0
        },
        "clusterName": {
          "type": "string",
          "minLength": 0
        },
        "controlPlaneNodes": {
          "type": "integer",
          "minLength": 0
        },
        "controlPlaneVMType": {
          "type": "string",
          "minLength": 0
        },
        "kubernetesVersion": {
          "type": "string",
          "minLength": 0
        },
        "location": {
          "type": "string",
          "minLength": 0
        },
        "mgmtClusterKubeConfigPath": {
          "type": "string",
          "minLength": 0
        },
        "nodeVMType": {
          "type": "string",
          "minLength": 0
        },
        "nodes": {
          "type": "integer",
          "minLength": 0
        },
        "resourceGroup": {
          "type": "string",
          "minLength": 0
        },
        "sshPublicKey": {
          "type": "string",
          "minLength": 0
        },
        "subscriptionID": {
          "type": "string",
          "minLength": 0
        },
        "tenantID": {
          "type": "string",
          "minLength": 0
        },
        "vnetName": {
          "type": "string",
          "minLength": 0
        }
      }
    },
    "error": {
      "type": "object",
      "required": [
        "code",
        "message"
      ],
      "properties": {
        "code": {
          "type": "integer",
          "format": "int64"
        },
        "message": {
          "type": "string"
        }
      }
    }
  },
  "securityDefinitions": {
    "basic": {
      "type": "basic"
    }
  }
}`))
}
