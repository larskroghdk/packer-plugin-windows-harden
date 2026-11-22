package provisioner

import (
	"encoding/xml"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
)

// TestUnattendedSetupEscapesXML evaluates the actual vSphere template arguments
// and checks that XML metacharacters survive setup without altering the password.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestUnattendedSetupEscapesXML(t *testing.T) {
	data, err := os.ReadFile("../examples/vsphere-iso/windows-harden.pkr.hcl")
	if err != nil {
		t.Fatal(err)
	}
	file, diagnostics := hclsyntax.ParseConfig(data, "windows-harden.pkr.hcl", hcl.InitialPos)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	password, image := `test-p<&>"'assword`, `Windows <Enterprise> & Test`
	ctx := &hcl.EvalContext{
		Variables: map[string]cty.Value{"var": cty.ObjectVal(map[string]cty.Value{
			"admin_password": cty.StringVal(password), "image_name": cty.StringVal(image),
		})},
		Functions: map[string]function.Function{"replace": stdlib.ReplaceFunc},
	}
	var templateVars cty.Value
	for _, block := range file.Body.(*hclsyntax.Body).Blocks {
		if block.Type != "source" {
			continue
		}
		content := block.Body.Attributes["cd_content"].Expr.(*hclsyntax.ObjectConsExpr)
		call := content.Items[0].ValueExpr.(*hclsyntax.FunctionCallExpr)
		templateVars, diagnostics = call.Args[1].Value(ctx)
		if diagnostics.HasErrors() {
			t.Fatal(diagnostics.Error())
		}
	}
	if templateVars == cty.NilVal {
		t.Fatal("unattended setup arguments were not found")
	}
	data, err = os.ReadFile("../examples/vsphere-iso/autounattend.pkrtpl.xml")
	if err != nil {
		t.Fatal(err)
	}
	template, diagnostics := hclsyntax.ParseTemplate(data, "autounattend.pkrtpl.xml", hcl.InitialPos)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	rendered, diagnostics := template.Value(&hcl.EvalContext{Variables: templateVars.AsValueMap()})
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	decoder := xml.NewDecoder(strings.NewReader(rendered.AsString()))
	values := map[string]int{}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if start, ok := token.(xml.StartElement); ok && start.Name.Local == "Value" {
			var value string
			if err := decoder.DecodeElement(&value, &start); err != nil {
				t.Fatal(err)
			}
			values[value]++
		}
	}
	if values[password] != 2 || values[image] != 1 {
		t.Fatalf("rendered values did not preserve credentials and image name: %v", values)
	}
}
