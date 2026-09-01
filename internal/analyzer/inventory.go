package analyzer

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func InventoryForRoot(root string) (Inventory, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return Inventory{}, err
	}
	result := Inventory{RootREADMEExcluded: true}
	err = filepath.WalkDir(absolute, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == absolute {
			return nil
		}
		name := entry.Name()
		if entry.IsDir() {
			if name == ".git" || name == "cache" || name == "vendor" || name == "toolchain" {
				return filepath.SkipDir
			}
			result.DescendantDirs++
			return nil
		}
		if !entry.Type().IsRegular() || (path == filepath.Join(absolute, "README.md")) {
			return nil
		}
		result.DescendantFiles++
		extension := filepath.Ext(name)
		if extension != ".go" && extension != ".gooo" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lines := physicalLines(raw)
		if extension == ".go" {
			result.GoFiles++
			result.GoPhysicalLines += lines
		} else {
			result.GoooFiles++
			result.GoooPhysicalLines += lines
		}
		return nil
	})
	return result, err
}

func physicalLines(raw []byte) int {
	if len(raw) == 0 {
		return 0
	}
	lines := strings.Count(string(raw), "\n")
	if raw[len(raw)-1] != '\n' {
		lines++
	}
	return lines
}
