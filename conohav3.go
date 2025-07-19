package conohav3

import "errors"

// identityRequest is the top-level payload sent to the Identity v3.
type identityRequest struct {
	Auth auth `json:"auth"`
}

// auth authentication credentials (identity) and scope (scope).
type auth struct {
	Identity identity `json:"identity"`
	Scope    scope    `json:"scope"`
}

// identity describes how the client will authenticate.
// In ConoHa VPS VER.3.0, only support the "password" method.
type identity struct {
	Methods  []string `json:"methods"`
	Password password `json:"password"`
}

// password nests the concrete user credentials used by the password auth method.
type password struct {
	User user `json:"user"`
}

// user holds the API User ID and password that will be verified by the Identity service.
type user struct {
	ID       string `json:"id"`
	Password string `json:"password"`
}

// scope specifies which tenant the issued token should be scoped to.
type scope struct {
	Project project `json:"project"`
}

// project identifies the target tenant by UUID.
type project struct {
	ID string `json:"id"`
}

// domainListResponse is returned by `GET /v1/domains` and contains all DNS zones (domains) owned by the project.
type domainListResponse struct {
	Domains []domain `json:"domains"`
}

// domain represents a single hosted DNS zone.
type domain struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

// recordListResponse is returned by `GET /v1/domains/{domain_uuid}/records` and lists every record in the zone.
type recordListResponse struct {
	Records []conohaDNSRecord `json:"records"`
}

// conohaDNSRecord represents a DNS record inside a zone.
// NOTE: TTL is marked with `omitempty` because:
// - The ConoHa API accepts `ttl` in POST (record creation), even though it's undocumented.
// - The API rejects `ttl` in PUT (record update), returning HTTP 400.
// This design ensures TTL is included only when non-zero (typically during creation).
type conohaDNSRecord struct {
	UUID string `json:"uuid,omitempty"`
	Name string `json:"name"`
	Type string `json:"type"`
	Data string `json:"data"`
	TTL  int    `json:"ttl,omitempty"` // TTL is readonly on update — see note above.
}

var errRecordNotFound = errors.New("Record not found")
var errRecordNotSupported = errors.New("Record Type is not supported")
