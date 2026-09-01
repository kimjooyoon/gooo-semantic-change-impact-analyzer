package analyzer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type SemanticIR struct {
	Schema         string     `json:"schema"`
	SourcePath     string     `json:"source_path"`
	SourceDigest   string     `json:"source_digest"`
	ContractPath   string     `json:"contract_path"`
	ContractDigest string     `json:"contract_digest"`
	Release        string     `json:"release"`
	Activities     []Activity `json:"activities"`
}

func Compile(sourcePath, contractPath, outputIR, outputGo string) error {
	if !filepath.IsAbs(outputIR) || !filepath.IsAbs(outputGo) {
		return fmt.Errorf("compile outputs must be absolute caller-owned paths")
	}
	if err := ensureOutsideRepository(outputIR); err != nil {
		return err
	}
	if err := ensureOutsideRepository(outputGo); err != nil {
		return err
	}
	sourceRaw, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}
	contract, contractRaw, err := LoadContract(contractPath)
	if err != nil {
		return err
	}
	_ = contract
	source, err := ParseSource(sourceRaw)
	if err != nil {
		return err
	}
	ir := SemanticIR{
		Schema: IRSchema, SourcePath: filepath.ToSlash(sourcePath), SourceDigest: DigestBytes(sourceRaw),
		ContractPath: filepath.ToSlash(contractPath), ContractDigest: DigestBytes(contractRaw),
		Release: source.Release, Activities: append([]Activity(nil), source.Activities...),
	}
	irRaw, err := JSON(ir)
	if err != nil {
		return err
	}
	binding := struct {
		SourceDigest   string   `json:"source_digest"`
		ContractDigest string   `json:"contract_digest"`
		Release        string   `json:"release"`
		Activities     []string `json:"activities"`
	}{SourceDigest: ir.SourceDigest, ContractDigest: ir.ContractDigest, Release: ir.Release, Activities: append([]string(nil), RequiredActivities...)}
	bindingRaw, err := json.Marshal(binding)
	if err != nil {
		return err
	}
	generated := "// Code generated from the released .gooo declaration; DO NOT EDIT.\npackage generated\n\nconst ReleasedSemanticGraphBinding = `" + string(bindingRaw) + "`\n"
	if err := WriteCallerFile(outputIR, append(irRaw, '\n')); err != nil {
		return err
	}
	return WriteCallerFile(outputGo, []byte(generated))
}

func WriteCallerFile(path string, raw []byte) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("caller output must be an absolute path")
	}
	if err := ensureOutsideRepository(path); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func ensureOutsideRepository(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	root := findRepositoryRoot()
	if root != "" && isWithin(root, absolute) {
		return fmt.Errorf("caller-owned output must be outside repository")
	}
	return nil
}

func EnsureCallerDirectory(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("output directory must be an absolute caller-owned path")
	}
	if err := ensureOutsideRepository(path); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("output path must be a directory")
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("caller-owned output directory must be empty")
	}
	return nil
}

func findRepositoryRoot() string {
	current, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if info, err := os.Stat(filepath.Join(current, ".git")); err == nil && info.IsDir() {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

func isWithin(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && (relative == "." || (relative != ".." && !startsWithParent(relative)))
}

func startsWithParent(relative string) bool {
	return len(relative) >= 3 && relative[:3] == ".."+string(filepath.Separator)
}
