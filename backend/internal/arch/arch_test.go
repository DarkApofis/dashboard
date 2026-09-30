// Package arch holds an executable version of the layering rules.
//
// The dependency direction is the one decision that silently rots first: one
// pragmatic import of pgx from a use case, and the domain stops being testable
// without a database. Asserting it here means the rule is enforced by `go test`
// rather than by whoever happens to review the pull request.
package arch

import (
	"os/exec"
	"strings"
	"testing"
)

// forbiddenImports lists, per layer, the import prefixes it must never pull in
// (directly or transitively).
var forbiddenImports = []struct {
	layer     string
	pattern   string
	forbidden []string
	reason    string
}{
	{
		layer:   "domain",
		pattern: "paymentlinks/internal/domain/...",
		forbidden: []string{
			"net/http",
			"github.com/jackc/",
			"github.com/go-chi/",
			"encoding/json",
			"paymentlinks/internal/application",
			"paymentlinks/internal/infrastructure",
			"paymentlinks/internal/transport",
			"paymentlinks/internal/config",
		},
		reason: "the domain is the innermost layer: it must not know about HTTP, SQL, JSON or any outer layer",
	},
	{
		layer:   "application",
		pattern: "paymentlinks/internal/application/...",
		forbidden: []string{
			"net/http",
			"github.com/jackc/",
			"github.com/go-chi/",
			"paymentlinks/internal/infrastructure",
			"paymentlinks/internal/transport",
		},
		reason: "use cases depend on ports, never on the adapters that implement them",
	},
	{
		layer:   "transport",
		pattern: "paymentlinks/internal/transport/...",
		forbidden: []string{
			"github.com/jackc/",
			"paymentlinks/internal/infrastructure",
		},
		reason: "handlers call use cases; they must not reach the database",
	},
}

func TestLayerDependencies(t *testing.T) {
	for _, rule := range forbiddenImports {
		t.Run(rule.layer, func(t *testing.T) {
			deps := transitiveDeps(t, rule.pattern)

			for _, dep := range deps {
				for _, forbidden := range rule.forbidden {
					if strings.HasPrefix(dep, forbidden) {
						t.Errorf("%s depends on %q, which is forbidden: %s", rule.layer, dep, rule.reason)
					}
				}
			}
		})
	}
}

// transitiveDeps asks the toolchain for the full dependency closure of the
// matching packages, so an import smuggled in two hops away is caught too.
func transitiveDeps(t *testing.T, pattern string) []string {
	t.Helper()

	cmd := exec.Command("go", "list", "-deps", pattern)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps %s: %v\n%s", pattern, err, output)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	deps := make([]string, 0, len(lines))
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			deps = append(deps, trimmed)
		}
	}
	return deps
}
