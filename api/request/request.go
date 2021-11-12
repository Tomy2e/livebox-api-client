package request

// Request contains information that form an API request. The struct
// is json-encoded before being sent to the API.
type Request struct {
	Service    string                 `json:"service"`
	Method     string                 `json:"method"`
	Parameters map[string]interface{} `json:"parameters"`
}

// New returns a new request object.
func New(service, method string, params map[string]interface{}) *Request {
	return &Request{
		Service:    service,
		Method:     method,
		Parameters: params,
	}
}
