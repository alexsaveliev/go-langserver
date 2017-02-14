package langserver

import (
	"bytes"
	"context"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"strings"

	"github.com/sourcegraph/go-langserver/pkg/lsp"
	"github.com/sourcegraph/jsonrpc2"
	"golang.org/x/tools/go/loader"
)

func (h *LangHandler) handleTextDocumentSignatureHelp(ctx context.Context, conn jsonrpc2.JSONRPC2, req *jsonrpc2.Request, params lsp.TextDocumentPositionParams) (*lsp.SignatureHelp, error) {
	fset, _, nodes, program, pkg, err := h.typecheck(ctx, conn, params.TextDocument.URI, params.Position)
	if err != nil {
		if _, ok := err.(*invalidNodeError); !ok {
			return nil, err
		}
	}

	call := callExpr(fset, nodes)
	if call == nil {
		return nil, nil
	}

	signature, parameters, doc := funcInfo(program, pkg, call.Fun)
	if signature == "" {
		return nil, nil
	}

	info := lsp.SignatureInformation{Label: signature, Documentation: doc}
	info.Parameters = make([]lsp.ParameterInformation, len(parameters))
	for i := 0; i < len(parameters); i++ {
		info.Parameters[i] = lsp.ParameterInformation{Label: parameters[i]}
	}
	activeParameter := len(info.Parameters)
	if activeParameter > 0 {
		activeParameter = activeParameter - 1
	}
	numArguments := len(call.Args)
	if activeParameter > numArguments {
		activeParameter = numArguments
	}

	return &lsp.SignatureHelp{Signatures: []lsp.SignatureInformation{info}, ActiveSignature: 0, ActiveParameter: activeParameter}, nil
}

// callExpr climbs AST tree up until call expression
func callExpr(fset *token.FileSet, nodes []ast.Node) *ast.CallExpr {
	for _, node := range nodes {
		callExpr, ok := node.(*ast.CallExpr)
		if ok {
			return callExpr
		}
	}
	return nil
}

// shortObject returns shorthand object notation
func shortObject(o types.Object) string {
	return types.ObjectString(o, func(*types.Package) string {
		return ""
	})
}

// funcInfo returns declaration about given function call expression
func funcInfo(prog *loader.Program, pkg *loader.PackageInfo, node ast.Node) (signature string, parameters []string, documentation string) {
	_, path, _ := prog.PathEnclosingInterval(node.Pos(), node.Pos())
	for i := 0; i < len(path); i++ {
		callExpr, ok := path[i].(*ast.CallExpr)
		if !ok {
			continue
		}
		fDecl := funcDecl(prog, pkg, callExpr)
		if fDecl == nil {
			return "", nil, ""
		}
		var doc string
		if fDecl.Doc != nil {
			doc = fDecl.Doc.Text()
		}
		parameters := parametersAsString(fDecl.Type.Params, pkg)
		// do not print function body, docs, or parameters:
		// we don't need body or docs and want parameters to be in form "name type"
		// to enable highlighting of Nth parameter in IDE
		// without custom parameters formatter we may have troubles dealing with
		// "foo,bar baz" form of parameters declaration
		clone := ast.FuncDecl{Recv: fDecl.Recv,
			Name: fDecl.Name,
			Type: &ast.FuncType{Params: &ast.FieldList{},
				Results: fDecl.Type.Results}}
		signature := nodeAsString(&clone, prog.Fset)
		return strings.Replace(signature, "()", "("+strings.Join(parameters, ", ")+")", 1), parameters, doc
	}
	return "", nil, ""
}

// ident looks for first ident node in the given path.
// handles idents and selectors
func ident(prog *loader.Program, pkg *loader.PackageInfo, node ast.Node) *ast.Ident {
	_, path, _ := prog.PathEnclosingInterval(node.Pos(), node.End())
	for i := 0; i < 4 && i < len(path); i++ {
		switch t := path[i].(type) {
		case *ast.Ident:
			return t
		case *ast.SelectorExpr:
			return t.Sel
		}
	}
	return nil
}

// funcDecl returns AST node where function referenced by the given call expression was defined
func funcDecl(prog *loader.Program, pkg *loader.PackageInfo, node *ast.CallExpr) *ast.FuncDecl {
	ident := ident(prog, pkg, node.Fun)
	if ident == nil {
		return nil
	}
	o := pkg.ObjectOf(ident)
	if o == nil || !o.Pos().IsValid() {
		return nil
	}
	_, path, _ := prog.PathEnclosingInterval(o.Pos(), o.Pos())
	for i := 0; i < len(path); i++ {
		decl, ok := path[i].(*ast.FuncDecl)
		if ok {
			return decl
		}
	}
	return nil
}

// asString returns string representation of given node
func nodeAsString(node ast.Node, fset *token.FileSet) string {
	buf := &bytes.Buffer{}
	cfg := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 4}
	err := cfg.Fprint(buf, fset, node)
	if err != nil {
		return ""
	}
	return buf.String()
}

// parametersAsString returns string representations of function parameters
func parametersAsString(fields *ast.FieldList, pkg *loader.PackageInfo) []string {
	labels := make([]string, 0, len(fields.List)*2)
	for _, field := range fields.List {
		t := pkg.TypeOf(field.Type).String()
		for _, name := range field.Names {
			labels = append(labels, name.String()+" "+t)
		}
	}
	return labels
}
