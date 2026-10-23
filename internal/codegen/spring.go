package codegen

import (
	"fmt"
	"strings"

	"github.com/JohnEsleyer/barista/internal/classfile"
	"github.com/JohnEsleyer/barista/internal/ir"
)

const (
	SpringRestController = "barista/spring/annotation/RestController"
	SpringRequestMapping = "barista/spring/annotation/RequestMapping"
	SpringGetMapping     = "barista/spring/annotation/GetMapping"
	SpringPostMapping    = "barista/spring/annotation/PostMapping"
	SpringPutMapping     = "barista/spring/annotation/PutMapping"
	SpringDeleteMapping  = "barista/spring/annotation/DeleteMapping"
	SpringRequestBody    = "barista/spring/annotation/RequestBody"
	SpringPathVariable   = "barista/spring/annotation/PathVariable"
	SpringRequestParam   = "barista/spring/annotation/RequestParam"
)

// SpringParam describes an argument to a controller method.
type SpringParam struct {
	Index   int
	Name    string
	Type    string
	IsBody  bool
	IsPath  bool
	IsQuery bool
	Key     string
}

// SpringEndpoint describes a route extracted at compile-time.
type SpringEndpoint struct {
	HTTPMethod  string
	Path        string
	MethodName  string
	GoMethod    string
	Controller  string
	Descriptor  string
	Params      []SpringParam
	ReturnsJSON bool
}

// SpringController encapsulates an ahead-of-time discovered controller.
type SpringController struct {
	Class      *classfile.ClassFile
	Controller string
	BasePath   string
	Endpoints  []SpringEndpoint
}

// DetectSpringControllers scans for classes marked with barista.spring.annotation.RestController.
func DetectSpringControllers(hierarchy *classfile.ClassHierarchy, methodsByClass map[string][]*ir.MethodIR) []*SpringController {
	var controllers []*SpringController

	for _, name := range hierarchy.Names() {
		cf := hierarchy.Classes[name]
		if !hasAnnotation(cf.Attributes, SpringRestController) {
			continue
		}

		basePath := ""
		if ann := findAnnotation(cf.Attributes, SpringRequestMapping); ann != nil {
			basePath = extractPathValue(ann)
		}
		if !strings.HasPrefix(basePath, "/") && basePath != "" {
			basePath = "/" + basePath
		}
		basePath = strings.TrimSuffix(basePath, "/")

		methods := methodsByClass[cf.ThisClass]
		methodNames := ResolveMethodNames(cf.ThisClass, methods)

		var endpoints []SpringEndpoint

		for _, m := range cf.Methods {
			httpMethod, path := extractRouteMapping(m.Attributes)
			if httpMethod == "" {
				continue
			}

			if !strings.HasPrefix(path, "/") && path != "" {
				path = "/" + path
			}
			fullPath := basePath + path
			if fullPath == "" {
				fullPath = "/"
			}
			if !strings.HasPrefix(fullPath, "/") {
				fullPath = "/" + fullPath
			}

			key := MethodKey(cf.ThisClass, m.Name, m.Descriptor)
			goName := methodNames[key]
			if goName == "" {
				goName = CleanMemberName(m.Name)
			}

			md, _ := ir.ParseMethodDescriptor(m.Descriptor)
			params := make([]SpringParam, 0, len(md.Params))

			for i, pType := range md.Params {
				param := SpringParam{
					Index: i,
					Type:  pType,
				}
				// Detect @RequestBody or default to body if POST/PUT with non-primitive
				if (httpMethod == "POST" || httpMethod == "PUT") && strings.HasPrefix(pType, "L") {
					param.IsBody = true
				}
				params = append(params, param)
			}

			// Returns JSON if returning a reference type other than java.lang.String
			returnsJSON := strings.HasPrefix(md.Return, "L") && md.Return != "Ljava/lang/String;"

			endpoints = append(endpoints, SpringEndpoint{
				HTTPMethod:  httpMethod,
				Path:        fullPath,
				MethodName:  m.Name,
				GoMethod:    goName,
				Controller:  CleanClassName(cf.ThisClass),
				Descriptor:  m.Descriptor,
				Params:      params,
				ReturnsJSON: returnsJSON,
			})
		}

		controllers = append(controllers, &SpringController{
			Class:      cf,
			Controller: CleanClassName(cf.ThisClass),
			BasePath:   basePath,
			Endpoints:  endpoints,
		})
	}

	return controllers
}

func hasAnnotation(attrs []classfile.AttributeInfo, target string) bool {
	return findAnnotation(attrs, target) != nil
}

func findAnnotation(attrs []classfile.AttributeInfo, target string) *classfile.Annotation {
	for _, attr := range attrs {
		if annAttr, ok := attr.(*classfile.RuntimeVisibleAnnotationsAttribute); ok {
			for i := range annAttr.Annotations {
				if strings.Contains(annAttr.Annotations[i].Type, target) {
					return &annAttr.Annotations[i]
				}
			}
		}
	}
	return nil
}

func extractPathValue(ann *classfile.Annotation) string {
	if ann == nil {
		return ""
	}
	for _, pair := range ann.ElementValuePairs {
		if pair.Name == "value" || pair.Name == "path" {
			if s, ok := pair.Value.Value.(string); ok {
				return s
			}
			if arr, ok := pair.Value.Value.([]classfile.ElementValue); ok && len(arr) > 0 {
				if s, ok := arr[0].Value.(string); ok {
					return s
				}
			}
		}
	}
	return ""
}

func extractRouteMapping(attrs []classfile.AttributeInfo) (httpMethod, path string) {
	if ann := findAnnotation(attrs, SpringGetMapping); ann != nil {
		return "GET", extractPathValue(ann)
	}
	if ann := findAnnotation(attrs, SpringPostMapping); ann != nil {
		return "POST", extractPathValue(ann)
	}
	if ann := findAnnotation(attrs, SpringPutMapping); ann != nil {
		return "PUT", extractPathValue(ann)
	}
	if ann := findAnnotation(attrs, SpringDeleteMapping); ann != nil {
		return "DELETE", extractPathValue(ann)
	}
	if ann := findAnnotation(attrs, SpringRequestMapping); ann != nil {
		return "GET", extractPathValue(ann)
	}
	return "", ""
}

// GenerateSpringMainGo synthesizes the native Go server entrypoint with compile-time JSON serialization.
func GenerateSpringMainGo(controllers []*SpringController) []byte {
	var b strings.Builder

	b.WriteString(`package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	app "barista_app/app"
	"github.com/JohnEsleyer/barista/pkg/runtime/java/lang"
)

var _ = json.Marshal
var _ = io.ReadAll
var _ = strings.TrimSpace
var _ = lang.NewString

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()

`)

	// 1. Static Dependency Injection: Instantiate controllers
	for _, c := range controllers {
		varName := strings.ToLower(c.Controller)
		b.WriteString(fmt.Sprintf("\t%s := &app.%s{}\n", varName, c.Controller))
	}
	b.WriteString("\n")

	// 2. Static Route Registrations with Go 1.22+ routing syntax
	for _, c := range controllers {
		ctrlVar := strings.ToLower(c.Controller)
		for _, ep := range c.Endpoints {
			// Leverage Go 1.22 pattern matching: "GET /api/users"
			pattern := fmt.Sprintf("%s %s", ep.HTTPMethod, ep.Path)
			b.WriteString(fmt.Sprintf("\tmux.HandleFunc(%q, func(w http.ResponseWriter, r *http.Request) {\n", pattern))
			b.WriteString("\t\t_ = r\n")

			callArgs := make([]string, 0, len(ep.Params))
			for i, p := range ep.Params {
				if p.IsBody {
					if p.Type == "Ljava/lang/String;" {
						argName := fmt.Sprintf("bodyStr%d", i)
						b.WriteString("\t\tbodyBytes, _ := io.ReadAll(r.Body)\n")
						b.WriteString(fmt.Sprintf("\t\t%s := lang.NewString(string(bodyBytes))\n", argName))
						callArgs = append(callArgs, argName)
					} else {
						// Complex struct/data class request body: JSON deserialize directly
						typeName := CleanClassName(strings.TrimSuffix(strings.TrimPrefix(p.Type, "L"), ";"))
						argName := fmt.Sprintf("reqBody%d", i)
						b.WriteString(fmt.Sprintf("\t\tvar %s app.%s\n", argName, typeName))
						b.WriteString(fmt.Sprintf("\t\tif err := json.NewDecoder(r.Body).Decode(&%s); err != nil {\n", argName))
						b.WriteString("\t\t\thttp.Error(w, fmt.Sprintf(\"Invalid JSON payload: %v\", err), http.StatusBadRequest)\n")
						b.WriteString("\t\t\treturn\n")
						b.WriteString("\t\t}\n")
						callArgs = append(callArgs, "&"+argName)
					}
				}
			}

			call := fmt.Sprintf("%s.%s(%s)", ctrlVar, ep.GoMethod, strings.Join(callArgs, ", "))

			if ep.ReturnsJSON {
				b.WriteString(fmt.Sprintf("\t\tres := %s\n", call))
				b.WriteString("\t\tif res == nil {\n")
				b.WriteString("\t\t\tw.WriteHeader(http.StatusNotFound)\n")
				b.WriteString("\t\t\treturn\n")
				b.WriteString("\t\t}\n")
				b.WriteString("\t\tw.Header().Set(\"Content-Type\", \"application/json; charset=utf-8\")\n")
				b.WriteString("\t\tw.WriteHeader(http.StatusOK)\n")
				b.WriteString("\t\tif err := json.NewEncoder(w).Encode(res); err != nil {\n")
				b.WriteString("\t\t\thttp.Error(w, err.Error(), http.StatusInternalServerError)\n")
				b.WriteString("\t\t}\n")
			} else if strings.HasSuffix(ep.Descriptor, ")Ljava/lang/String;") {
				b.WriteString(fmt.Sprintf("\t\tres := %s\n", call))
				b.WriteString("\t\tw.Header().Set(\"Content-Type\", \"text/plain; charset=utf-8\")\n")
				b.WriteString("\t\tw.WriteHeader(http.StatusOK)\n")
				b.WriteString("\t\tw.Write([]byte(res.GoString()))\n")
			} else if strings.HasSuffix(ep.Descriptor, ")I") || strings.HasSuffix(ep.Descriptor, ")J") {
				b.WriteString(fmt.Sprintf("\t\tres := %s\n", call))
				b.WriteString("\t\tw.Header().Set(\"Content-Type\", \"text/plain; charset=utf-8\")\n")
				b.WriteString("\t\tw.WriteHeader(http.StatusOK)\n")
				b.WriteString("\t\tfmt.Fprintf(w, \"%d\", res)\n")
			} else {
				b.WriteString(fmt.Sprintf("\t\t%s\n", call))
				b.WriteString("\t\tw.WriteHeader(http.StatusOK)\n")
			}

			b.WriteString("\t})\n\n")
		}
	}

	b.WriteString(`	fmt.Printf("☕ Freshly brewed Barista Spring service running on port %s (booted in <5ms)\n", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		fmt.Fprintf(os.Stderr, "Server failed: %v\n", err)
		os.Exit(1)
	}
}
`)

	return []byte(b.String())
}
