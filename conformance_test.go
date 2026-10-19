package synthient

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// schemaTypes maps each schema in .sdkgen/openapi.json to the Go type that
// models it. When the spec changes, update this map and the types together.
var schemaTypes = map[string]reflect.Type{
	"AccountAPIKey":            reflect.TypeFor[AccountAPIKey](),
	"AccountInfoResponse":      reflect.TypeFor[Account](),
	"ActivityStatsBlock":       reflect.TypeFor[ActivityStats](),
	"AnonymizerStreamEntry":    reflect.TypeFor[AnonymizerEvent](),
	"ASNEntry":                 reflect.TypeFor[ASNCount](),
	"CountryEntry":             reflect.TypeFor[CountryCount](),
	"DeviceInfo":               reflect.TypeFor[Device](),
	"DomainLookupResponse":     reflect.TypeFor[DomainInfo](),
	"ExportListItem":           reflect.TypeFor[Export](),
	"ExportListResponse":       reflect.TypeFor[ExportPage](),
	"ExportMetaResponse":       reflect.TypeFor[ExportMeta](),
	"HoneypotADBStreamEntry":   reflect.TypeFor[HeliosADBEvent](),
	"HoneypotDNSStreamEntry":   reflect.TypeFor[HeliosDNSEvent](),
	"HoneypotHTTPSStreamEntry": reflect.TypeFor[HeliosHTTPSEvent](),
	"HoneypotHTTPStreamEntry":  reflect.TypeFor[HeliosHTTPEvent](),
	"HoneypotMeta":             reflect.TypeFor[HeliosMeta](),
	"IntelligenceInfo":         reflect.TypeFor[Intelligence](),
	"IpBatchResponse":          reflect.TypeFor[batchResponse](),
	"IpProvidersResponse":      reflect.TypeFor[IPInfo](),
	"JA4TStreamEntry":          reflect.TypeFor[JA4TEvent](),
	"LocationInfo":             reflect.TypeFor[Location](),
	"LookupQuota":              reflect.TypeFor[LookupQuota](),
	"NetworkInfo":              reflect.TypeFor[Network](),
	"Organization":             reflect.TypeFor[Organization](),
	"PortEntry":                reflect.TypeFor[PortCount](),
	"ProblemDetails":           reflect.TypeFor[problem](),
	"ProjectedColumn":          reflect.TypeFor[ParquetColumn](),
	"ProviderEntry":            reflect.TypeFor[Provider](),
	"ProxyStreamEntry":         reflect.TypeFor[ProxyEvent](),
	"RecentEvent":              reflect.TypeFor[DomainEvent](),
	"Schema":                   reflect.TypeFor[ParquetSchema](),
	"Stats":                    reflect.TypeFor[DomainStats](),
	"SubdomainEntry":           reflect.TypeFor[SubdomainCount](),
	"TimeSeriesEntry":          reflect.TypeFor[TimeSeriesPoint](),
	"TLSClientHello":           reflect.TypeFor[TLSClientHello](),
	"TLSCodeName":              reflect.TypeFor[TLSCodeName](),
	"TLSExtension":             reflect.TypeFor[TLSExtension](),
	"TorrentFileEntry":         reflect.TypeFor[TorrentFile](),
	"TorrentPeerEntry":         reflect.TypeFor[TorrentPeer](),
	"TorrentStreamEntry":       reflect.TypeFor[TorrentEvent](),
	"UniqueIPsBlock":           reflect.TypeFor[UniqueIPs](),
	"ValidationProblemDetails": reflect.TypeFor[problem](),
}

// unmodeled lists schemas the SDK encodes without a named type. LookupIPs
// sends IpBatchRequest as a map, and TestLookupIPs checks its body.
var unmodeled = []string{"IpBatchRequest"}

// liveFormats overrides spec formats that disagree with the live API. Helios
// streams send tunnel IDs above the spec's int64 maximum.
var liveFormats = map[string]string{
	"HoneypotDNSStreamEntry.tunnel_id":   "uint64",
	"HoneypotHTTPSStreamEntry.tunnel_id": "uint64",
	"HoneypotHTTPStreamEntry.tunnel_id":  "uint64",
}

type specSchema struct {
	Ref            string                `json:"$ref"`
	Type           string                `json:"type"`
	Format         string                `json:"format"`
	Properties     map[string]specSchema `json:"properties"`
	Items          *specSchema           `json:"items"`
	AllOf          []specSchema          `json:"allOf"`
	Enum           []string              `json:"enum"`
	ExtensibleEnum []string              `json:"x-extensible-enum"`
}

func (s specSchema) enumValues() []string {
	if s.Items != nil {
		return s.Items.enumValues()
	}
	return slices.Concat(s.Enum, s.ExtensibleEnum)
}

type spec struct {
	Paths      map[string]map[string]json.RawMessage `json:"paths"`
	Components struct {
		Schemas map[string]specSchema `json:"schemas"`
	} `json:"components"`
}

func loadSpec(t *testing.T) spec {
	t.Helper()
	data, err := os.ReadFile(".sdkgen/openapi.json")
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	var s spec
	err = json.Unmarshal(data, &s)
	if err != nil {
		t.Fatalf("decode spec: %v", err)
	}
	return s
}

func TestSpecSchemasAreModeled(t *testing.T) {
	t.Parallel()
	s := loadSpec(t)

	for _, name := range slices.Sorted(maps.Keys(s.Components.Schemas)) {
		if slices.Contains(unmodeled, name) {
			continue
		}
		typ, ok := schemaTypes[name]
		if !ok {
			t.Errorf("schema %s has no Go type in schemaTypes", name)
			continue
		}
		checkFields(t, name, s.Components.Schemas[name], typ)
	}
	for _, name := range slices.Concat(slices.Collect(maps.Keys(schemaTypes)), unmodeled) {
		_, ok := s.Components.Schemas[name]
		if !ok {
			t.Errorf("%s is listed in conformance_test.go but is no longer in the spec", name)
		}
	}
}

func checkFields(t *testing.T, where string, schema specSchema, typ reflect.Type) {
	t.Helper()
	fields := jsonFields(typ)
	for _, name := range slices.Sorted(maps.Keys(schema.Properties)) {
		prop := schema.Properties[name]
		format, ok := liveFormats[where+"."+name]
		if ok {
			prop.Format = format
		}
		field, ok := fields[name]
		if !ok {
			t.Errorf("%s.%s has no field in %s", where, name, typ)
			continue
		}
		if !compatible(prop, field.Type) {
			t.Errorf("%s.%s is %s %s in the spec but %s in %s", where, name, prop.Type, prop.Format, field.Type, typ)
		}
		if prop.Ref == "" && len(prop.Properties) > 0 && field.Type.Kind() == reflect.Struct {
			checkFields(t, where+"."+name, prop, field.Type)
		}
	}
}

func jsonFields(typ reflect.Type) map[string]reflect.StructField {
	fields := map[string]reflect.StructField{}
	for field := range typ.Fields() {
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name != "" && name != "-" {
			fields[name] = field
		}
	}
	return fields
}

func compatible(schema specSchema, typ reflect.Type) bool {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if len(schema.AllOf) == 1 {
		return compatible(schema.AllOf[0], typ)
	}
	if schema.Ref != "" {
		name, ok := strings.CutPrefix(schema.Ref, "#/components/schemas/")
		return ok && schemaTypes[name] == typ
	}
	if typ == reflect.TypeFor[json.RawMessage]() {
		return true
	}
	switch schema.Type {
	case "string":
		if schema.Format == "byte" {
			return typ == reflect.TypeFor[[]byte]()
		}
		return typ.Kind() == reflect.String || typ == reflect.TypeFor[netip.Addr]() || typ == reflect.TypeFor[time.Time]()
	case "integer":
		switch schema.Format {
		case "int64":
			return typ.Kind() == reflect.Int64
		case "uint64":
			return typ.Kind() == reflect.Uint64
		}
		return slices.Contains([]reflect.Kind{reflect.Int, reflect.Int32, reflect.Int64}, typ.Kind())
	case "number":
		return typ.Kind() == reflect.Float64
	case "boolean":
		return typ.Kind() == reflect.Bool
	case "array":
		return typ.Kind() == reflect.Slice && schema.Items != nil && compatible(*schema.Items, typ.Elem())
	case "object":
		return typ.Kind() == reflect.Struct || typ.Kind() == reflect.Map
	default:
		return false
	}
}

func TestSpecEnumsAreModeled(t *testing.T) {
	t.Parallel()
	s := loadSpec(t)
	constants := stringConstants(t)

	for _, name := range slices.Sorted(maps.Keys(s.Components.Schemas)) {
		typ, ok := schemaTypes[name]
		if !ok {
			continue
		}
		fields := jsonFields(typ)
		schema := s.Components.Schemas[name]
		for _, prop := range slices.Sorted(maps.Keys(schema.Properties)) {
			values := schema.Properties[prop].enumValues()
			field, ok := fields[prop]
			// A single-value enum, such as a stream's protocol, is a constant
			// and stays a plain string.
			if len(values) < 2 || !ok {
				continue
			}
			enum := field.Type
			if enum.Kind() == reflect.Slice {
				enum = enum.Elem()
			}
			if enum.PkgPath() == "" {
				t.Errorf("%s.%s is an enum in the spec but %s in %s", name, prop, field.Type, typ)
				continue
			}
			got := slices.Sorted(slices.Values(constants[enum.Name()]))
			want := slices.Sorted(slices.Values(values))
			if !slices.Equal(got, want) {
				t.Errorf("%s constants are %q, but the spec's %s.%s values are %q", enum.Name(), got, name, prop, want)
			}
		}
	}
}

// stringConstants returns the values of the package's typed string constants,
// keyed by type name.
func stringConstants(t *testing.T) map[string][]string {
	t.Helper()
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	constants := map[string][]string{}
	fset := token.NewFileSet()
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok || len(value.Values) != 1 {
					continue
				}
				typ, ok := value.Type.(*ast.Ident)
				if !ok {
					continue
				}
				lit, ok := value.Values[0].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				text, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatal(err)
				}
				constants[typ.Name] = append(constants[typ.Name], text)
			}
		}
	}
	return constants
}

func TestSchemaReferencesMatchExactTypes(t *testing.T) {
	t.Parallel()
	schema := specSchema{Ref: "#/components/schemas/NetworkInfo"}
	for _, typ := range []reflect.Type{reflect.TypeFor[Network](), reflect.TypeFor[*Network]()} {
		if !compatible(schema, typ) {
			t.Errorf("NetworkInfo rejected %s", typ)
		}
	}
	for _, typ := range []reflect.Type{reflect.TypeFor[Location](), reflect.TypeFor[*Location](), reflect.TypeFor[json.RawMessage]()} {
		if compatible(schema, typ) {
			t.Errorf("NetworkInfo accepted unrelated type %s", typ)
		}
	}
	if compatible(specSchema{Ref: "#/components/schemas/Unknown"}, reflect.TypeFor[Network]()) {
		t.Error("unknown schema reference was accepted")
	}
}

func TestSpecOperationsAreCovered(t *testing.T) {
	t.Parallel()
	s := loadSpec(t)
	c, err := NewClient(testAPIKey)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	covered := map[string]bool{
		"GET /account/me":             true,
		"GET /health":                 true,
		"GET /lookup/domain/{domain}": true,
		"GET /lookup/ip/{ip_address}": true,
		"POST /lookup/ips":            true,
	}
	feedPaths := []string{
		c.Proxies.path, c.Anonymizers.path, c.Torrents.path, c.JA4T.path,
		c.HeliosHTTP.path, c.HeliosHTTPS.path, c.HeliosDNS.path, c.HeliosADB.path,
	}
	for _, path := range feedPaths {
		for _, suffix := range []string{"/export", "/export/{date}", "/export/{date}/meta", "/export/{date}/{hour}", "/export/{date}/{hour}/meta", "/stream"} {
			covered["GET "+path+suffix] = true
		}
	}

	operations := map[string]bool{}
	for path, methods := range s.Paths {
		for method := range methods {
			operations[strings.ToUpper(method)+" "+path] = true
		}
	}
	for _, op := range slices.Sorted(maps.Keys(operations)) {
		if !covered[op] {
			t.Errorf("spec operation %s is not implemented", op)
		}
	}
	for _, op := range slices.Sorted(maps.Keys(covered)) {
		if !operations[op] {
			t.Errorf("implemented operation %s is not in the spec", op)
		}
	}
}
