package skill

import (
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dargstack/dargstack/v4/internal/version"
)

//go:embed SKILL.md
var bundledSkill string

const (
	skillName     = "dargstack"
	skillFileName = "SKILL.md"
	metaFileName  = ".dargstack-meta"
	skillsDirName = ".agents/skills"
)

// Meta stores version and hash of an installed skill.
type Meta struct {
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
}

// BundledContent returns the embedded SKILL.md content.
func BundledContent() string { return bundledSkill }

// BundledHash returns the sha256 hex digest of the embedded skill content.
func BundledHash() string {
	h := sha256.Sum256([]byte(bundledSkill))
	return fmt.Sprintf("%x", h)
}

// BundledVersion returns the dargstack version associated with the bundled skill.
func BundledVersion() string { return version.Version }

// SkillPath returns the path to the skill directory.
// If project is true, uses .agents/skills/ in the current working directory.
// Otherwise, uses ~/.agents/skills/.
func SkillPath(project bool) (string, error) {
	if project {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get working directory: %w", err)
		}
		return filepath.Join(cwd, skillsDirName, skillName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(home, skillsDirName, skillName), nil
}

// SkillFilePath returns the path to SKILL.md in the given skill directory.
func SkillFilePath(skillDir string) string {
	return filepath.Join(skillDir, skillFileName)
}

// MetaFilePath returns the path to .dargstack-meta in the given skill directory.
func MetaFilePath(skillDir string) string {
	return filepath.Join(skillDir, metaFileName)
}

// ReadMeta reads the metadata file from the skill directory.
// Returns nil Meta and nil error if the file doesn't exist.
func ReadMeta(skillDir string) (*Meta, error) {
	path := MetaFilePath(skillDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read metadata: %w", err)
	}
	var meta Meta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parse metadata: %w", err)
	}
	return &meta, nil
}

// WriteMeta writes version and hash metadata to the skill directory.
func WriteMeta(skillDir, ver, hash string) error {
	meta := Meta{Version: ver, SHA256: hash}
	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	path := MetaFilePath(skillDir)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write metadata: %w", err)
	}
	return nil
}

// IsInstalled returns true if the skill directory exists and contains SKILL.md.
func IsInstalled(skillDir string) bool {
	_, err := os.Stat(SkillFilePath(skillDir))
	return err == nil
}

// Install installs or updates the skill at the given directory.
// Returns (updated bool, userModified bool, error).
// updated is true if the file was written.
// userModified is true if the existing file's hash differs from both the
// previous bundled hash and the current bundled hash.
func Install(skillDir string) (updated, userModified bool, err error) {
	existingMeta, err := ReadMeta(skillDir)
	if err != nil {
		return false, false, err
	}

	bHash := BundledHash()
	skillPath := SkillFilePath(skillDir)

	// Check for user modifications first.
	if existingMeta != nil {
		currentData, readErr := os.ReadFile(skillPath)
		if readErr == nil {
			currentHash := fmt.Sprintf("%x", sha256.Sum256(currentData))
			if currentHash != existingMeta.SHA256 && currentHash != bHash {
				return false, true, nil
			}
		}
	}

	// Check if already up to date and skill file still exists.
	if existingMeta != nil && existingMeta.SHA256 == bHash {
		if _, statErr := os.Stat(skillPath); statErr == nil {
			return false, false, nil
		}
	}

	// Create directory if needed.
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return false, false, fmt.Errorf("create skill directory: %w", err)
	}

	// Write skill file.
	if err := os.WriteFile(skillPath, []byte(bundledSkill), 0o644); err != nil {
		return false, false, fmt.Errorf("write skill file: %w", err)
	}

	// Write metadata.
	if err := WriteMeta(skillDir, BundledVersion(), bHash); err != nil {
		return false, false, err
	}

	return true, false, nil
}

// Uninstall removes the skill directory and metadata.
// Returns error if the skill is not installed or metadata is missing.
func Uninstall(skillDir string) error {
	meta, err := ReadMeta(skillDir)
	if err != nil {
		return err
	}
	if meta == nil {
		return fmt.Errorf("skill is not installed (no metadata found)")
	}
	// Allow uninstall to clean up partial installs even if SKILL.md is missing.
	if err := os.RemoveAll(skillDir); err != nil {
		return fmt.Errorf("remove skill directory: %w", err)
	}
	return nil
}

// Update updates the skill if the bundled version differs from the installed one.
// Returns (updated bool, error). Errors if not already installed.
func Update(skillDir string) (bool, error) {
	meta, err := ReadMeta(skillDir)
	if err != nil {
		return false, err
	}
	if meta == nil {
		return false, fmt.Errorf("skill is not installed at %s", skillDir)
	}

	updated, userModified, err := Install(skillDir)
	if err != nil {
		return false, err
	}
	if !userModified {
		return updated, nil
	}
	// Explicit update should overwrite user modifications.
	bHash := BundledHash()
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return false, fmt.Errorf("create skill directory: %w", err)
	}
	if err := os.WriteFile(SkillFilePath(skillDir), []byte(bundledSkill), 0o644); err != nil {
		return false, fmt.Errorf("write skill file: %w", err)
	}
	if err := WriteMeta(skillDir, BundledVersion(), bHash); err != nil {
		return false, err
	}
	return true, nil
}

// StatusInfo holds the status of an installed skill.
type StatusInfo struct {
	Installed     bool
	Location      string
	BundledVer    string
	BundledHash   string
	InstalledVer  string
	InstalledHash string
	UpToDate      bool
	UserModified  bool
}

// Status returns information about the installed skill.
func Status(skillDir string) (*StatusInfo, error) {
	info := &StatusInfo{
		Location:    skillDir,
		BundledVer:  BundledVersion(),
		BundledHash: BundledHash(),
	}

	if !IsInstalled(skillDir) {
		return info, nil
	}

	meta, err := ReadMeta(skillDir)
	if err != nil {
		return nil, err
	}
	if meta != nil {
		info.Installed = true
		info.InstalledVer = meta.Version
		info.InstalledHash = meta.SHA256
		info.UpToDate = meta.SHA256 == BundledHash()

		// Check for user modifications.
		currentData, readErr := os.ReadFile(SkillFilePath(skillDir))
		if readErr == nil {
			currentHash := fmt.Sprintf("%x", sha256.Sum256(currentData))
			info.UserModified = currentHash != meta.SHA256 && currentHash != BundledHash()
		}
	}

	return info, nil
}
