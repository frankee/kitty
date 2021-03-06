package grpc_transport


import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"kitty/protoc-gen-kitty/descriptor"
	"github.com/grpc-ecosystem/grpc-gateway/utilities"
	"github.com/golang/glog"
)

type param struct {
	*descriptor.File
	Imports           []descriptor.GoPackage
	UseRequestContext bool
}

type binding struct {
	*descriptor.Binding
}

// HasQueryParam determines if the binding needs parameters in query string.
//
// It sometimes returns true even though actually the binding does not need.
// But it is not serious because it just results in a small amount of extra codes generated.
func (b binding) HasQueryParam() bool {
	if b.Body != nil && len(b.Body.FieldPath) == 0 {
		return false
	}
	fields := make(map[string]bool)
	for _, f := range b.Method.RequestType.Fields {
		fields[f.GetName()] = true
	}
	if b.Body != nil {
		delete(fields, b.Body.FieldPath.String())
	}
	for _, p := range b.PathParams {
		delete(fields, p.FieldPath.String())
	}
	return len(fields) > 0
}

func (b binding) QueryParamFilter() queryParamFilter {
	var seqs [][]string
	if b.Body != nil {
		seqs = append(seqs, strings.Split(b.Body.FieldPath.String(), "."))
	}
	for _, p := range b.PathParams {
		seqs = append(seqs, strings.Split(p.FieldPath.String(), "."))
	}
	return queryParamFilter{utilities.NewDoubleArray(seqs)}
}

// queryParamFilter is a wrapper of utilities.DoubleArray which provides String() to output DoubleArray.Encoding in a stable and predictable format.
type queryParamFilter struct {
	*utilities.DoubleArray
}

func (f queryParamFilter) String() string {
	encodings := make([]string, len(f.Encoding))
	for str, enc := range f.Encoding {
		encodings[enc] = fmt.Sprintf("%q: %d", str, enc)
	}
	e := strings.Join(encodings, ", ")
	return fmt.Sprintf("&utilities.DoubleArray{Encoding: map[string]int{%s}, Base: %#v, Check: %#v}", e, f.Base, f.Check)
}

type trailerParams struct {
	Services          []*descriptor.Service
	UseRequestContext bool
}

func applyTemplate(p param) (string, error) {
	w := bytes.NewBuffer(nil)
	if err := headerTemplate.Execute(w, p); err != nil {
		return "", err
	}

	var targetServices []*descriptor.Service
	for _, svc := range p.Services {
		//var methodWithBindingsSeen bool
		svcName := strings.Title(*svc.Name)
		svc.Name = &svcName

		if err := endPointTemplate.Execute(w, p); err != nil {
			glog.Fatal("failed endpoint")
			return "", err
		}

		//if methodWithBindingsSeen {
		targetServices = append(targetServices, svc)
		//}
	}
	if len(targetServices) == 0 {
		glog.Fatal("0 endpoint")
		return "", errNoTargetService
	}

	//tp := trailerParams{
	//	Services:          targetServices,
	//	UseRequestContext: p.UseRequestContext,
	//}
	//if err := trailerTemplate.Execute(w, tp); err != nil {
	//	return "", err
	//}
	return w.String(), nil
}

var (
	headerTemplate = template.Must(template.New("header").Parse(`
// Code generated by protoc-gen-grpc-gateway. DO NOT EDIT.
// source: {{.GetName}}

/*
Package {{.GoPkg.Name}} is a reverse proxy.

It translates gRPC into RESTful JSON APIs.
*/
package {{.GoPkg.Name}}
import (
	{{range $i := .Imports}}{{if $i.Standard}}{{$i | printf "%s\n"}}{{end}}{{end}}

	{{range $i := .Imports}}{{if not $i.Standard}}{{$i | printf "%s\n"}}{{end}}{{end}}
)

var _ codes.Code
var _ io.Reader
var _ status.Status
var _ = runtime.String
var _ = utilities.NewDoubleArray
`))

	gRpcServerTemplate = template.Must(template.New("gRpcServer").Parse(`

{{range $svc := .Services}}

// Set collects all of the endpoints that compose an {{$svc.GetName}} service. It's meant to
// be used as a helper struct, to collect all of the endpoints into a single
// parameter.
type gRpc{{$svc.GetName}}Server struct {
	{{range $m := $svc.Methods}}
		{{$m.GetLowerName}} kittygrpc.Server
	{{end}}
}

// NewGRPCServer makes a set of endpoints available as a gRPC {{$svc.GetName}}Server.
func NewGRpc{{$svc.GetName}}Server(svc {{$svc.GetName}}Server) {{$svc.GetName}}Server {
	return &gRpc{{$svc.GetName}}Server{
	{{range $m := $svc.Methods}}
		{{$m.GetLowerName}}: kittygrpc.NewServer(
			{{$m.GetName}}Endpoint,
			nil
        ),
	{{end}}
	}
}

{{range $m := $svc.Methods}}

func (s gRpc{{$svc.GetName}}Server) {{$m.GetName}}(ctx context.Context, request *{{$m.RequestType.GoType $m.Service.File.GoPkg.Path}}) (*{{$m.ResponseType.GoType $m.Service.File.GoPkg.Path}}, error) {
	_, rep, err := s.sum(ctx, req)
	if err != nil {
		return nil, err
	}
	return rep.(*pb.SumReply), nil

	return endPoints.{{$m.GetName}}EndPoint(ctx, req)
}

{{end}}

{{range $m := $svc.Methods}}

// Make{{$m.GetName}}Endpoint constructs a {{$m.GetName}} endpoint wrapping the service.
func Make{{$m.GetName}}Endpoint(s {{$svc.GetName}}Server) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		return s.{{$m.GetName}}(ctx, request)
	}
}

{{end}}

{{end}}

`))


	gRpcClientTemplate = template.Must(template.New("gRpcClient").Parse(`

{{range $svc := .Services}}

// Set collects all of the endpoints that compose an {{$svc.GetName}} service. It's meant to
// be used as a helper struct, to collect all of the endpoints into a single
// parameter.
type gRpc{{$svc.GetName}}Server struct {
	{{range $m := $svc.Methods}}
		{{$m.GetLowerName}} kittygrpc.Server
	{{end}}
}

// NewGRPCServer makes a set of endpoints available as a gRPC {{$svc.GetName}}Server.
func NewGRpc{{$svc.GetName}}Server(svc {{$svc.GetName}}Server) {{$svc.GetName}}Server {
	return &gRpc{{$svc.GetName}}Server{
	{{range $m := $svc.Methods}}
		{{$m.GetLowerName}}: kittygrpc.NewServer(
			{{$m.GetName}}Endpoint,
			nil
        ),
	{{end}}
	}
}

{{range $m := $svc.Methods}}

func (s gRpc{{$svc.GetName}}Server) {{$m.GetName}}(ctx context.Context, request *{{$m.RequestType.GoType $m.Service.File.GoPkg.Path}}) (*{{$m.ResponseType.GoType $m.Service.File.GoPkg.Path}}, error) {
	_, rep, err := s.sum(ctx, req)
	if err != nil {
		return nil, err
	}
	return rep.(*pb.SumReply), nil

	return endPoints.{{$m.GetName}}EndPoint(ctx, req)
}

{{end}}

{{range $m := $svc.Methods}}

// Make{{$m.GetName}}Endpoint constructs a {{$m.GetName}} endpoint wrapping the service.
func Make{{$m.GetName}}Endpoint(s {{$svc.GetName}}Server) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		return s.{{$m.GetName}}(ctx, request)
	}
}

{{end}}

{{end}}

`))
)
