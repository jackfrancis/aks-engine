// Code generated by go-swagger; DO NOT EDIT.

package operations

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"net/http"

	"github.com/go-openapi/runtime"

	"github.com/Azure/aks-engine/pkg/swagger/models"
)

// CreateClusterOKCode is the HTTP code returned for type CreateClusterOK
const CreateClusterOKCode int = 200

/*CreateClusterOK create cluster response

swagger:response createClusterOK
*/
type CreateClusterOK struct {

	/*
	  In: Body
	*/
	Payload *models.CreateData `json:"body,omitempty"`
}

// NewCreateClusterOK creates CreateClusterOK with default headers values
func NewCreateClusterOK() *CreateClusterOK {

	return &CreateClusterOK{}
}

// WithPayload adds the payload to the create cluster o k response
func (o *CreateClusterOK) WithPayload(payload *models.CreateData) *CreateClusterOK {
	o.Payload = payload
	return o
}

// SetPayload sets the payload to the create cluster o k response
func (o *CreateClusterOK) SetPayload(payload *models.CreateData) {
	o.Payload = payload
}

// WriteResponse to the client
func (o *CreateClusterOK) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.WriteHeader(200)
	if o.Payload != nil {
		payload := o.Payload
		if err := producer.Produce(rw, payload); err != nil {
			panic(err) // let the recovery middleware deal with this
		}
	}
}

/*CreateClusterDefault unexpected error

swagger:response createClusterDefault
*/
type CreateClusterDefault struct {
	_statusCode int

	/*
	  In: Body
	*/
	Payload *models.Error `json:"body,omitempty"`
}

// NewCreateClusterDefault creates CreateClusterDefault with default headers values
func NewCreateClusterDefault(code int) *CreateClusterDefault {
	if code <= 0 {
		code = 500
	}

	return &CreateClusterDefault{
		_statusCode: code,
	}
}

// WithStatusCode adds the status to the create cluster default response
func (o *CreateClusterDefault) WithStatusCode(code int) *CreateClusterDefault {
	o._statusCode = code
	return o
}

// SetStatusCode sets the status to the create cluster default response
func (o *CreateClusterDefault) SetStatusCode(code int) {
	o._statusCode = code
}

// WithPayload adds the payload to the create cluster default response
func (o *CreateClusterDefault) WithPayload(payload *models.Error) *CreateClusterDefault {
	o.Payload = payload
	return o
}

// SetPayload sets the payload to the create cluster default response
func (o *CreateClusterDefault) SetPayload(payload *models.Error) {
	o.Payload = payload
}

// WriteResponse to the client
func (o *CreateClusterDefault) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.WriteHeader(o._statusCode)
	if o.Payload != nil {
		payload := o.Payload
		if err := producer.Produce(rw, payload); err != nil {
			panic(err) // let the recovery middleware deal with this
		}
	}
}
