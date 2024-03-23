package generator

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
	log "github.com/sirupsen/logrus" // nolint: depguard

	"github.com/juanjiTech/protoc-gen-grpc-gateway-ts/data"
	"github.com/juanjiTech/protoc-gen-grpc-gateway-ts/registry"
	"github.com/pkg/errors"
)

// TypeScriptGRPCGatewayGenerator is the protobuf generator for typescript
type TypeScriptGRPCGatewayGenerator struct {
	Registry *registry.Registry
	// EnableStylingCheck enables both eslint and tsc check for the generated code
	// This option will only turn on in integration test to ensure the readability in
	// the generated code.
	EnableStylingCheck bool
}

const (
	// EnableStylingCheckOption is the option name for EnableStylingCheck
	EnableStylingCheckOption = "enable_styling_check"
)

// New returns an initialised generator
func New(paramsMap map[string]string) (*TypeScriptGRPCGatewayGenerator, error) {
	registry, err := registry.NewRegistry(paramsMap)
	if err != nil {
		return nil, errors.Wrap(err, "error instantiating a new registry")
	}

	enableStylingCheck := false
	enableStylingCheckVal, ok := paramsMap[EnableStylingCheckOption]
	if ok {
		// default to true if not disabled specifi
		enableStylingCheck = enableStylingCheckVal == "true"
	}

	return &TypeScriptGRPCGatewayGenerator{
		Registry:           registry,
		EnableStylingCheck: enableStylingCheck,
	}, nil
}

// Generate take a code generator request and returns a response. it analyses request with registry and use the generated data to render ts files
func (t *TypeScriptGRPCGatewayGenerator) Generate(req *plugin.CodeGeneratorRequest) (*plugin.CodeGeneratorResponse, error) {
	resp := &plugin.CodeGeneratorResponse{}

	filesData, err := t.Registry.Analyse(req)
	if err != nil {
		return nil, errors.Wrap(err, "error analysing proto files")
	}
	tmpl := GetTemplate(t.Registry)
	log.Debugf("files to generate %v", req.GetFileToGenerate())

	needToGenerateFetchModule := false
	// feed fileData into rendering process
	for _, fileData := range filesData {
		fileData.EnableStylingCheck = t.EnableStylingCheck
		if !t.Registry.IsFileToGenerate(fileData.Name) {
			log.Debugf("file %s is not the file to generate, skipping", fileData.Name)
			continue
		}

		log.Debugf("generating file for %s", fileData.TSFileName)
		generated, err := t.generateFile(fileData, tmpl)
		if err != nil {
			return nil, errors.Wrap(err, "error generating file")
		}
		resp.File = append(resp.File, generated)
		needToGenerateFetchModule = needToGenerateFetchModule || fileData.Services.NeedsFetchModule()
	}

	if needToGenerateFetchModule {
		// generate fetch module
		fetchTmpl := GetFetchModuleTemplate()
		log.Debugf("generate fetch template")
		generatedFetch, err := t.generateFetchModule(fetchTmpl)
		if err != nil {
			return nil, errors.Wrap(err, "error generating fetch module")
		}

		if generatedFetch != nil {
			resp.File = append(resp.File, generatedFetch)
		}
	}
	return resp, nil
}

func (t *TypeScriptGRPCGatewayGenerator) generateFile(fileData *data.File, tmpl *template.Template) (*plugin.CodeGeneratorResponse_File, error) {
	w := bytes.NewBufferString("")

	if fileData.IsEmpty() {
		w.Write([]byte("export default {}\n"))
	} else {
		err := tmpl.Execute(w, fileData)
		if err != nil {
			return nil, errors.Wrapf(err, "error generating ts file for %s", fileData.Name)
		}
	}

	fileName := fileData.TSFileName
	content := strings.TrimSpace(w.String())

	return &plugin.CodeGeneratorResponse_File{
		Name:           &fileName,
		InsertionPoint: nil,
		Content:        &content,
	}, nil
}

func (t *TypeScriptGRPCGatewayGenerator) generateFetchModule(tmpl *template.Template) (*plugin.CodeGeneratorResponse_File, error) {
	w := new(bytes.Buffer)
	fileName := filepath.ToSlash(filepath.Join(t.Registry.FetchModuleDirectory, t.Registry.FetchModuleFilename))

	err := tmpl.Execute(w, &data.File{EnableStylingCheck: t.EnableStylingCheck})
	if err != nil {
		return nil, errors.Wrapf(err, "error generating fetch module at %s", fileName)
	}

	content := strings.TrimSpace(w.String())

	// check if the file exists
	stat, err := os.Stat(fileName)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	// if the file exists, load the content and check if it's the same as the generated content
	var originContent []byte
	if stat != nil {
		originContent, err = os.ReadFile(fileName)
		if err != nil {
			log.Debugf("error reading file %s, err %s", fileName, err)
			return nil, err
		}
	}
	if string(originContent) == content {
		log.Debugf("fetch module content is the same as the existing file, skipping")
		return nil, nil
	}
	return &plugin.CodeGeneratorResponse_File{
		Name:           &fileName,
		InsertionPoint: nil,
		Content:        &content,
	}, nil
}
