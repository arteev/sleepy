package sleepy

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

//The definition of methods http
const (
	GET    = "GET"
	POST   = "POST"
	PUT    = "PUT"
	DELETE = "DELETE"
	HEAD   = "HEAD"
	PATCH  = "PATCH"
)

//Errors
var (
	ErrNotDefinedResource = errors.New("You must add at least one resource to this API")
)

// GetSupported is the interface that provides the Get
// method a resource must support to receive HTTP GETs.
type GetSupported interface {
	Get(url.Values, http.Header) (int, interface{}, http.Header)
}

//GetSupportedRequest is the interface that provides the Get
// method a resource must support to receive HTTP GETs
type GetSupportedRequest interface {
	Get(*http.Request) (int, interface{}, http.Header)
}

// PostSupported is the interface that provides the Post
// method a resource must support to receive HTTP POSTs.
type PostSupported interface {
	Post(url.Values, http.Header) (int, interface{}, http.Header)
}

// PostSupportedRequest is the interface that provides the Post
// method a resource must support to receive HTTP POSTs.
type PostSupportedRequest interface {
	Post(*http.Request) (int, interface{}, http.Header)
}

// PutSupported is the interface that provides the Put
// method a resource must support to receive HTTP PUTs.
type PutSupported interface {
	Put(url.Values, http.Header) (int, interface{}, http.Header)
}

// PutSupportedRequest is the interface that provides the Put
// method a resource must support to receive HTTP PUTs.
type PutSupportedRequest interface {
	Put(*http.Request) (int, interface{}, http.Header)
}

// DeleteSupported is the interface that provides the Delete
// method a resource must support to receive HTTP DELETEs.
type DeleteSupported interface {
	Delete(url.Values, http.Header) (int, interface{}, http.Header)
}

// DeleteSupportedRequest is the interface that provides the Delete
// method a resource must support to receive HTTP DELETEs.
type DeleteSupportedRequest interface {
	Delete(*http.Request) (int, interface{}, http.Header)
}

// HeadSupported is the interface that provides the Head
// method a resource must support to receive HTTP HEADs.
type HeadSupported interface {
	Head(url.Values, http.Header) (int, interface{}, http.Header)
}

// HeadSupportedRequest is the interface that provides the Head
// method a resource must support to receive HTTP HEADs.
type HeadSupportedRequest interface {
	Head(*http.Request) (int, interface{}, http.Header)
}

// PatchSupported is the interface that provides the Patch
// method a resource must support to receive HTTP PATCHs.
type PatchSupported interface {
	Patch(url.Values, http.Header) (int, interface{}, http.Header)
}

// PatchSupportedRequest is the interface that provides the Patch
// method a resource must support to receive HTTP PATCHs.
type PatchSupportedRequest interface {
	Patch(*http.Request) (int, interface{}, http.Header)
}

// An API manages a group of resources by routing requests
// to the correct method on a matching resource and marshalling
// the returned data to JSON for the HTTP response.
//
// You can instantiate multiple APIs on separate ports. Each API
// will manage its own set of resources.
type API struct {
	mux            *http.ServeMux
	muxInitialized bool
	once           sync.Once
	marshal        map[string]Marshaler
}

//A Marshaler it is type for set up the option of  serialization  API
type Marshaler func(v interface{}, prefix, indent string) ([]byte, error)

//A Option using for options for API
type Option func(a *API)

//WithMarshaler using for setup a Marshaler for API. Name = Content-Type
func WithMarshaler(name string, m Marshaler) Option {
	return func(a *API) {
		a.marshal[name] = m
	}
}

// NewAPI allocates and returns a new API.
func NewAPI() *API {
	api := &API{
		marshal: map[string]Marshaler{"application/json": json.MarshalIndent},
	}

	return api
}

type (
	handlerType        func(url.Values, http.Header) (int, interface{}, http.Header)
	handlerTypeRequest func(*http.Request) (int, interface{}, http.Header)
)

func (api *API) detectHandler(request *http.Request, resource interface{}) (handler handlerTypeRequest) {

	decorateWithRequest := func(h handlerType) handlerTypeRequest {
		return func(r *http.Request) (int, interface{}, http.Header) {
			return h(r.Form, r.Header)
		}
	}

	switch request.Method {
	case GET:
		if resconcrete, ok := resource.(GetSupported); ok {
			handler = decorateWithRequest(resconcrete.Get)
		} else if resconcrete, ok := resource.(GetSupportedRequest); ok {
			handler = resconcrete.Get
		}
	case POST:
		if resconcrete, ok := resource.(PostSupported); ok {
			handler = decorateWithRequest(resconcrete.Post)
		} else if resconcrete, ok := resource.(PostSupportedRequest); ok {
			handler = resconcrete.Post
		}
	case PUT:
		if resconcrete, ok := resource.(PutSupported); ok {
			handler = decorateWithRequest(resconcrete.Put)
		} else if resconcrete, ok := resource.(PutSupportedRequest); ok {
			handler = resconcrete.Put
		}
	case DELETE:
		if resconcrete, ok := resource.(DeleteSupported); ok {
			handler = decorateWithRequest(resconcrete.Delete)
		} else if resconcrete, ok := resource.(DeleteSupportedRequest); ok {
			handler = resconcrete.Delete
		}
	case HEAD:
		if resconcrete, ok := resource.(HeadSupported); ok {
			handler = decorateWithRequest(resconcrete.Head)
		} else if resconcrete, ok := resource.(HeadSupportedRequest); ok {
			handler = resconcrete.Head
		}
	case PATCH:
		if resconcrete, ok := resource.(PatchSupported); ok {
			handler = decorateWithRequest(resconcrete.Patch)
		} else if resconcrete, ok := resource.(PatchSupportedRequest); ok {
			handler = resconcrete.Patch
		}
	}
	return
}

func (api *API) requestHandler(resource interface{}) http.HandlerFunc {
	return func(rw http.ResponseWriter, request *http.Request) {
		if request.ParseForm() != nil {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}
		var handler = api.detectHandler(request, resource)
		if handler == nil {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		//request.Context(),
		code, data, header := handler(request)
		content, err := api.doMarshal(data, header)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
		for name, values := range header {
			for _, value := range values {
				rw.Header().Add(name, value)
			}
		}
		rw.WriteHeader(code)
		rw.Write(content)
	}
}

func (api *API) doMarshal(data interface{}, header http.Header) (content []byte, err error) {
	contentType := "application/json"
	marshal := json.MarshalIndent
	if h, exists := header["Content-type"]; exists {
		contentType = h[0]
		if idx := strings.Index(contentType, ";"); idx != -1 {
			contentType = contentType[:idx]
		}
	}
	if api.marshal != nil {
		if m, exists := api.marshal[contentType]; exists {
			marshal = m
		}
	}
	content, err = marshal(data, "", "  ")
	return
}

// Mux returns the http.ServeMux used by an API. If a ServeMux has
// does not yet exist, a new one will be created and returned.
func (api *API) Mux() *http.ServeMux {
	api.once.Do(func() {
		api.mux = http.NewServeMux()
		api.muxInitialized = true
	})
	return api.mux
}

// AddResource adds a new resource to an API. The API will route
// requests that match one of the given paths to the matching HTTP
// method on the resource.
func (api *API) AddResource(resource interface{}, paths ...string) {
	for _, path := range paths {
		api.Mux().HandleFunc(path, api.requestHandler(resource))
	}
}

// AddResourceWithWrapper behaves exactly like AddResource but wraps
// the generated handler function with a give wrapper function to allow
// to hook in Gzip support and similar.
func (api *API) AddResourceWithWrapper(resource interface{}, wrapper func(handler http.HandlerFunc) http.HandlerFunc, paths ...string) {
	for _, path := range paths {
		api.Mux().HandleFunc(path, wrapper(api.requestHandler(resource)))
	}
}

// Start causes the API to begin serving requests on the given port with options.
func (api *API) Start(port int, opts ...Option) error {
	return api.StartAddr(fmt.Sprintf(":%d", port), opts...)
}

// StartAddr causes the API to begin serving requests on the address with options.
func (api *API) StartAddr(addr string, opts ...Option) error {
	if !api.muxInitialized {
		return ErrNotDefinedResource
	}
	for _, opt := range opts {
		opt(api)
	}
	return http.ListenAndServe(addr, api.Mux())
}
