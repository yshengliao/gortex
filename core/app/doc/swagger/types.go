package swagger

// OpenAPI represents the root OpenAPI 3.0 specification
type OpenAPI struct {
	OpenAPI    string                `json:"openapi"`
	Info       Info                  `json:"info"`
	Servers    []Server              `json:"servers,omitempty"`
	Paths      map[string]*PathItem  `json:"paths"`
	Components *Components           `json:"components,omitempty"`
	Security   []SecurityRequirement `json:"security,omitempty"`
	Tags       []Tag                 `json:"tags,omitempty"`
}

// Info represents API metadata
type Info struct {
	Title          string   `json:"title"`
	Description    string   `json:"description,omitempty"`
	TermsOfService string   `json:"termsOfService,omitempty"`
	Contact        *Contact `json:"contact,omitempty"`
	License        *License `json:"license,omitempty"`
	Version        string   `json:"version"`
}

// Contact represents contact information
type Contact struct {
	Name  string `json:"name,omitempty"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

// License represents license information
type License struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// Server represents a server
type Server struct {
	URL         string                    `json:"url"`
	Description string                    `json:"description,omitempty"`
	Variables   map[string]ServerVariable `json:"variables,omitempty"`
}

// ServerVariable represents a server variable
type ServerVariable struct {
	Enum        []string `json:"enum,omitempty"`
	Default     string   `json:"default"`
	Description string   `json:"description,omitempty"`
}

// PathItem represents operations available on a single path
type PathItem struct {
	Ref         string      `json:"$ref,omitempty"`
	Summary     string      `json:"summary,omitempty"`
	Description string      `json:"description,omitempty"`
	Get         *Operation  `json:"get,omitempty"`
	Put         *Operation  `json:"put,omitempty"`
	Post        *Operation  `json:"post,omitempty"`
	Delete      *Operation  `json:"delete,omitempty"`
	Options     *Operation  `json:"options,omitempty"`
	Head        *Operation  `json:"head,omitempty"`
	Patch       *Operation  `json:"patch,omitempty"`
	Trace       *Operation  `json:"trace,omitempty"`
	Servers     []Server    `json:"servers,omitempty"`
	Parameters  []Parameter `json:"parameters,omitempty"`
}

// Operation represents a single API operation on a path
type Operation struct {
	Tags         []string                        `json:"tags,omitempty"`
	Summary      string                          `json:"summary,omitempty"`
	Description  string                          `json:"description,omitempty"`
	ExternalDocs *ExternalDocumentation          `json:"externalDocs,omitempty"`
	OperationID  string                          `json:"operationId,omitempty"`
	Parameters   []Parameter                     `json:"parameters,omitempty"`
	RequestBody  *RequestBody                    `json:"requestBody,omitempty"`
	Responses    map[string]*Response            `json:"responses"`
	Callbacks    map[string]map[string]PathItem  `json:"callbacks,omitempty"`
	Deprecated   bool                            `json:"deprecated,omitempty"`
	Security     []SecurityRequirement           `json:"security,omitempty"`
	Servers      []Server                        `json:"servers,omitempty"`
}

// Parameter represents a parameter for an operation
type Parameter struct {
	Name            string  `json:"name"`
	In              string  `json:"in"`
	Description     string  `json:"description,omitempty"`
	Required        bool    `json:"required,omitempty"`
	Deprecated      bool    `json:"deprecated,omitempty"`
	AllowEmptyValue bool    `json:"allowEmptyValue,omitempty"`
	Style           string  `json:"style,omitempty"`
	Explode         bool    `json:"explode,omitempty"`
	AllowReserved   bool    `json:"allowReserved,omitempty"`
	Schema          *Schema `json:"schema,omitempty"`
	Example         any     `json:"example,omitempty"`
}

// RequestBody represents a request body
type RequestBody struct {
	Description string                `json:"description,omitempty"`
	Content     map[string]*MediaType `json:"content"`
	Required    bool                  `json:"required,omitempty"`
}

// MediaType represents a media type
type MediaType struct {
	Schema   *Schema              `json:"schema,omitempty"`
	Example  any                  `json:"example,omitempty"`
	Examples map[string]*Example  `json:"examples,omitempty"`
	Encoding map[string]*Encoding `json:"encoding,omitempty"`
}

// Response represents a response
type Response struct {
	Description string                `json:"description"`
	Headers     map[string]*Header    `json:"headers,omitempty"`
	Content     map[string]*MediaType `json:"content,omitempty"`
	Links       map[string]*Link      `json:"links,omitempty"`
}

// Schema represents a schema
type Schema struct {
	Ref                  string              `json:"$ref,omitempty"`
	Type                 string              `json:"type,omitempty"`
	Format               string              `json:"format,omitempty"`
	Title                string              `json:"title,omitempty"`
	Description          string              `json:"description,omitempty"`
	Default              any                 `json:"default,omitempty"`
	Maximum              *float64            `json:"maximum,omitempty"`
	ExclusiveMaximum     bool                `json:"exclusiveMaximum,omitempty"`
	Minimum              *float64            `json:"minimum,omitempty"`
	ExclusiveMinimum     bool                `json:"exclusiveMinimum,omitempty"`
	MaxLength            *int                `json:"maxLength,omitempty"`
	MinLength            *int                `json:"minLength,omitempty"`
	Pattern              string              `json:"pattern,omitempty"`
	MaxItems             *int                `json:"maxItems,omitempty"`
	MinItems             *int                `json:"minItems,omitempty"`
	UniqueItems          bool                `json:"uniqueItems,omitempty"`
	MaxProperties        *int                `json:"maxProperties,omitempty"`
	MinProperties        *int                `json:"minProperties,omitempty"`
	Required             []string            `json:"required,omitempty"`
	Enum                 []any               `json:"enum,omitempty"`
	Items                *Schema             `json:"items,omitempty"`
	Properties           map[string]*Schema  `json:"properties,omitempty"`
	AdditionalProperties any                 `json:"additionalProperties,omitempty"`
	AllOf                []*Schema           `json:"allOf,omitempty"`
	OneOf                []*Schema           `json:"oneOf,omitempty"`
	AnyOf                []*Schema           `json:"anyOf,omitempty"`
	Not                  *Schema             `json:"not,omitempty"`
	Discriminator        *Discriminator      `json:"discriminator,omitempty"`
	ReadOnly             bool                `json:"readOnly,omitempty"`
	WriteOnly            bool                `json:"writeOnly,omitempty"`
	XML                  *XML                `json:"xml,omitempty"`
	ExternalDocs         *ExternalDocumentation `json:"externalDocs,omitempty"`
	Example              any                 `json:"example,omitempty"`
	Deprecated           bool                `json:"deprecated,omitempty"`
}

// Components represents reusable components
type Components struct {
	Schemas         map[string]*Schema         `json:"schemas,omitempty"`
	Responses       map[string]*Response       `json:"responses,omitempty"`
	Parameters      map[string]*Parameter      `json:"parameters,omitempty"`
	Examples        map[string]*Example        `json:"examples,omitempty"`
	RequestBodies   map[string]*RequestBody    `json:"requestBodies,omitempty"`
	Headers         map[string]*Header         `json:"headers,omitempty"`
	SecuritySchemes map[string]*SecurityScheme `json:"securitySchemes,omitempty"`
	Links           map[string]*Link           `json:"links,omitempty"`
	Callbacks       map[string]map[string]PathItem `json:"callbacks,omitempty"`
}

// SecurityScheme represents a security scheme
type SecurityScheme struct {
	Type             string      `json:"type"`
	Description      string      `json:"description,omitempty"`
	Name             string      `json:"name,omitempty"`
	In               string      `json:"in,omitempty"`
	Scheme           string      `json:"scheme,omitempty"`
	BearerFormat     string      `json:"bearerFormat,omitempty"`
	Flows            *OAuthFlows `json:"flows,omitempty"`
	OpenIdConnectURL string      `json:"openIdConnectUrl,omitempty"`
}

// Other supporting types...

type SecurityRequirement map[string][]string

type Tag struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description,omitempty"`
	ExternalDocs *ExternalDocumentation `json:"externalDocs,omitempty"`
}

type ExternalDocumentation struct {
	Description string `json:"description,omitempty"`
	URL         string `json:"url"`
}

type Example struct {
	Summary       string `json:"summary,omitempty"`
	Description   string `json:"description,omitempty"`
	Value         any    `json:"value,omitempty"`
	ExternalValue string `json:"externalValue,omitempty"`
}

type Header struct {
	Description     string  `json:"description,omitempty"`
	Required        bool    `json:"required,omitempty"`
	Deprecated      bool    `json:"deprecated,omitempty"`
	AllowEmptyValue bool    `json:"allowEmptyValue,omitempty"`
	Schema          *Schema `json:"schema,omitempty"`
}

type Link struct {
	OperationRef string                 `json:"operationRef,omitempty"`
	OperationId  string                 `json:"operationId,omitempty"`
	Parameters   map[string]any         `json:"parameters,omitempty"`
	RequestBody  any                    `json:"requestBody,omitempty"`
	Description  string                 `json:"description,omitempty"`
	Server       *Server                `json:"server,omitempty"`
}

type Discriminator struct {
	PropertyName string            `json:"propertyName"`
	Mapping      map[string]string `json:"mapping,omitempty"`
}

type XML struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Prefix    string `json:"prefix,omitempty"`
	Attribute bool   `json:"attribute,omitempty"`
	Wrapped   bool   `json:"wrapped,omitempty"`
}

type Encoding struct {
	ContentType   string             `json:"contentType,omitempty"`
	Headers       map[string]*Header `json:"headers,omitempty"`
	Style         string             `json:"style,omitempty"`
	Explode       bool               `json:"explode,omitempty"`
	AllowReserved bool               `json:"allowReserved,omitempty"`
}

type OAuthFlows struct {
	Implicit          *OAuthFlow `json:"implicit,omitempty"`
	Password          *OAuthFlow `json:"password,omitempty"`
	ClientCredentials *OAuthFlow `json:"clientCredentials,omitempty"`
	AuthorizationCode *OAuthFlow `json:"authorizationCode,omitempty"`
}

type OAuthFlow struct {
	AuthorizationURL string            `json:"authorizationUrl,omitempty"`
	TokenURL         string            `json:"tokenUrl,omitempty"`
	RefreshURL       string            `json:"refreshUrl,omitempty"`
	Scopes           map[string]string `json:"scopes"`
}