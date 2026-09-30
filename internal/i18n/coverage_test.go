package i18n

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestEveryLiteralHasTranslation 扫描全仓非测试源码里 i18n.T("…") 的字符串字面量，
// 要求每条都在翻译表中登记，避免英文界面漏翻（T 查不到时会回退显示中文原文）。
// 以变量调用 i18n.T 的（如菜单选项表）由各自登记处保证。
func TestEveryLiteralHasTranslation(t *testing.T) {
	var missing []string
	fset := token.NewFileSet()
	err := filepath.WalkDir("../..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "dist" || strings.HasPrefix(d.Name(), ".")) && path != "../.." {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if key, ok := translationLiteral(n); ok {
				if _, found := registry[key]; !found {
					missing = append(missing, fset.Position(n.Pos()).String()+": "+key)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("%d untranslated i18n.T literals:\n%s", len(missing), strings.Join(missing, "\n"))
	}
}

// translationLiteral 识别 i18n.T("字面量") 调用并返回反引号/双引号解码后的 key。
func translationLiteral(n ast.Node) (string, bool) {
	call, ok := n.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return "", false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "T" {
		return "", false
	}
	if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "i18n" {
		return "", false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	key, err := strconv.Unquote(lit.Value)
	return key, err == nil
}
