package arch_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit/arch"
)

/*
TestAppBoundaries leaves app/models out of the import rule: sqlc
generates its queries on the database driver. The lint's import rules
still hold every hand-written file there. The LLM drivers under
app/drivers/llm are the one place the providers' SDKs may be imported:
each driver translates its SDK into the LLM contract and nothing above
it sees a provider type. Views are compiled by templ
onto its own runtime, so they may import templ, the framework and
each other, and nothing else. Packages are read as the default,
self-hosted build sees them, so files built only for the cloud are
left out: what remains must never reach the cloud-only packages under
app/cloud.
*/
func TestAppBoundaries(t *testing.T) {
	t.Parallel()

	m := arch.Load(t, module)

	t.Run("the product imports only the standard library, the framework and itself", func(t *testing.T) {
		t.Parallel()

		m.Expect("app/...", "routes/...", "config/...", "bootstrap/...", "lang/...", "resources/...").
			Except("app/models", "app/drivers/llm/...", "resources/views/...").
			IgnoringTests().
			ToOnlyImport(arch.Std, "framework/...", "app/...", "config/...", "routes/...", "database/migrations",
				"lang/...", "resources/...", "public")
	})

	t.Run("only code built for the cloud reaches cloud-only packages", func(t *testing.T) {
		t.Parallel()

		m.Expect("app/...", "routes/...", "config/...", "bootstrap/...", "resources/...").
			Except("app/cloud/...").
			NotToImport("app/cloud/...")
	})

	t.Run("only the bootstrap reaches the embedded public files", func(t *testing.T) {
		t.Parallel()

		m.Expect("app/...", "routes/...", "config/...", "lang/...", "resources/...").
			IgnoringTests().
			NotToImport("public")
	})

	t.Run("views import only templ's runtime, the framework and other views", func(t *testing.T) {
		t.Parallel()

		m.Expect("resources/views/...").
			IgnoringTests().
			ToOnlyImport(arch.Std, "framework/...", "resources/views/...", "github.com/a-h/templ",
				"github.com/a-h/templ/runtime")
	})

	t.Run("the product reaches drivers only through their managers", func(t *testing.T) {
		t.Parallel()

		m.Expect("app/...", "routes/...").NotToImport("framework/*/drivers/...")
	})

	t.Run("actions know nothing of HTTP", func(t *testing.T) {
		t.Parallel()

		m.Expect("app/actions/...").NotToImport("framework/http", "framework/http/...", "framework/routing")
	})

	t.Run("the application never imports what wires it", func(t *testing.T) {
		t.Parallel()

		m.Expect("app/...").NotToImport("routes/...", "bootstrap/...", "cmd/...", "tests/...", "database/factories")
		m.Expect("config/...").NotToImport("app/...", "routes/...")
	})
}

func TestRoleChecks(t *testing.T) {
	t.Parallel()

	t.Run("only the role table compares roles; everything else asks for permissions", func(t *testing.T) {
		t.Parallel()
		root := filepath.Join("..", "..", "app")

		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") ||
				strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, filepath.Join("auth", "roles.go")) {
				return err
			}

			for _, line := range roleComparisons(t, path) {
				t.Errorf("%s:%d compares a role; check a permission with authz.Can instead", path, line)
			}

			return nil
		})
		require.NoError(t, err)
	})
}

/*
roleComparisons returns the lines of the file where == or != has a role
on one side: a Role field, or one of the role constants.
*/
func roleComparisons(t *testing.T, path string) []int {
	t.Helper()

	files := token.NewFileSet()
	file, err := parser.ParseFile(files, path, nil, 0)
	require.NoError(t, err)

	var lines []int

	ast.Inspect(file, func(node ast.Node) bool {
		expr, ok := node.(*ast.BinaryExpr)
		if ok && (expr.Op == token.EQL || expr.Op == token.NEQ) && (isRole(expr.X) || isRole(expr.Y)) {
			lines = append(lines, files.Position(expr.Pos()).Line)
		}

		return true
	})

	return lines
}

/*
isRole reports whether the expression names a role: a field called
Role, or a constant starting with Role, such as enums.RoleOwner.
*/
func isRole(expr ast.Expr) bool {
	for {
		switch e := expr.(type) {
		case *ast.CallExpr:
			if len(e.Args) != 1 {
				return false
			}

			expr = e.Args[0]
		case *ast.SelectorExpr:
			return e.Sel.Name == "Role" || strings.HasPrefix(e.Sel.Name, "Role") && e.Sel.Name != "Roles"
		default:
			return false
		}
	}
}
