package benchmarks

import (
	"testing"

	"wut/internal/core"
	"wut/pkg/fuzzy"
)

// Fuzzy Matching Benchmarks
func BenchmarkMatchExact(b *testing.B) {
	m := fuzzy.NewMatcher(false, 3, 0.0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Match("test", "test")
	}
}

func BenchmarkMatchFuzzy(b *testing.B) {
	m := fuzzy.NewMatcher(false, 3, 0.0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Match("tset", "test")
	}
}

func BenchmarkMatchMultiple(b *testing.B) {
	m := fuzzy.NewMatcher(false, 3, 0.5)
	targets := []string{"git", "github", "gitlab", "docker", "kubectl", "npm", "yarn", "cargo"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.MatchMultiple("git", targets)
	}
}

// Parser Benchmarks
func BenchmarkParseSimple(b *testing.B) {
	p := core.NewParser()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Parse("git status")
	}
}

func BenchmarkParseComplex(b *testing.B) {
	p := core.NewParser()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Parse("docker run --name myapp -p 8080:80 -v /data:/app/data nginx:latest")
	}
}

func BenchmarkParsePiped(b *testing.B) {
	p := core.NewParser()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Parse("cat file.txt | grep test | wc -l")
	}
}
