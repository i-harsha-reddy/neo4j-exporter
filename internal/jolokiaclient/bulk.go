package jolokiaclient

// ReadRequest is one entry in a Jolokia bulk POST body. We only use the
// "read" operation form here; the "list", "exec", and "write" forms are
// out of scope for an exporter.
type ReadRequest struct {
	Type      string   `json:"type"` // always "read"
	MBean     string   `json:"mbean"`
	Attribute any      `json:"attribute,omitempty"` // string or []string
	Path      string   `json:"path,omitempty"`
	Target    *Target  `json:"target,omitempty"`
}

// Read constructs a single-attribute read request.
func Read(mbean, attr string) ReadRequest {
	return ReadRequest{Type: "read", MBean: mbean, Attribute: attr}
}

// ReadMulti constructs a read request for multiple attributes of one MBean.
// Jolokia returns a map keyed by attribute name in the response.
func ReadMulti(mbean string, attrs ...string) ReadRequest {
	return ReadRequest{Type: "read", MBean: mbean, Attribute: attrs}
}

// ReadAll constructs a read of all attributes of an MBean (no Attribute set).
// Useful for HeapMemoryUsage which returns a CompositeData with init/used/committed/max.
func ReadAll(mbean string) ReadRequest {
	return ReadRequest{Type: "read", MBean: mbean}
}

// Target lets a single Jolokia agent proxy to another JVM. We don't use
// it for the exporter (one Jolokia per JVM), but the field is part of
// the protocol.
type Target struct {
	URL      string `json:"url"`
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
}

// Response mirrors the Jolokia v2 response envelope. The Value field is
// kept as raw json.RawMessage and decoded by the caller because shape
// depends on which attribute(s) were requested.
type Response struct {
	Status    int            `json:"status"`
	Timestamp int64          `json:"timestamp"`
	Request   map[string]any `json:"request,omitempty"`
	Value     any            `json:"value,omitempty"`
	Error     string         `json:"error,omitempty"`
	ErrorType string         `json:"error_type,omitempty"`
}

// OK reports whether the per-request status is 200.
func (r Response) OK() bool { return r.Status == 200 }
