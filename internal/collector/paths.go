package collector

import (
	"path/filepath"
	"strings"
)

// IsNoisePath reports paths that are almost never useful in a standup
// (editor/agent metadata, dependencies, build output, OS junk).
// Real eng work under .github/ (workflows) is kept.
func IsNoisePath(p string) bool {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "./")
	if p == "" {
		return true
	}
	lower := strings.ToLower(filepath.ToSlash(p))
	base := strings.ToLower(filepath.Base(lower))

	switch base {
	case ".ds_store", "thumbs.db", ".gitignore", ".gitattributes",
		".editorconfig", ".npmrc", ".nvmrc", "package-lock.json",
		"yarn.lock", "pnpm-lock.yaml", "go.sum", "composer.lock",
		"gemfile.lock", "poetry.lock", "cargo.lock":
		return true
	case ".env", ".env.local", ".env.development", ".env.production", ".env.test",
		".dockerignore", ".prettierignore", ".eslintignore":
		return true
	}

	switch filepath.Ext(base) {
	case ".log", ".tmp", ".temp", ".swp", ".swo", ".bak", ".orig", ".pyc", ".pyo", ".class", ".o", ".a":
		return true
	}

	for _, part := range strings.Split(lower, "/") {
		switch part {
		case "node_modules", "vendor", "dist", "build", "out", "coverage",
			"__pycache__", ".next", ".nuxt", ".turbo", ".cache", ".parcel-cache",
			".gradle", ".terraform", "target", /* not "bin" — Go bin/ is often intentional */
			"obj", ".claude", ".cursor", ".vscode", ".idea", ".vs",
			".git", ".hg", ".svn", ".tox", ".mypy_cache", ".pytest_cache",
			".eslintcache", ".sass-cache", "xcuserdata", "deriveddata":
			return true
		}
	}
	return false
}

// MeaningfulFiles drops noise paths and returns clean relative paths.
func MeaningfulFiles(files []string) []string {
	var out []string
	for _, f := range files {
		f = strings.TrimSpace(f)
		if f == "" || strings.HasPrefix(f, "...and ") {
			continue
		}
		if IsNoisePath(f) {
			continue
		}
		out = append(out, f)
	}
	return out
}
