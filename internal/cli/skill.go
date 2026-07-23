package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/dargstack/dargstack/v4/internal/config"
	"github.com/dargstack/dargstack/v4/internal/logger"
	"github.com/dargstack/dargstack/v4/internal/skill"
)

var skillProject bool

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage the dargstack AI agent skill",
	Long: `Manage the dargstack AI agent skill.

The skill teaches AI agents about dargstack conventions: project structure,
spruce operators, secret templating, label semantics, and deploy workflow.

Install the skill globally (~/.agents/skills/dargstack/) or project-local
(.agents/skills/dargstack/) with --project.`,
}

var skillInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the dargstack agent skill",
	RunE:  runSkillInstall,
}

var skillUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall the dargstack agent skill",
	RunE:  runSkillUninstall,
}

var skillUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update the dargstack agent skill to the current bundled version",
	RunE:  runSkillUpdate,
}

var skillStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the status of the installed dargstack agent skill",
	RunE:  runSkillStatus,
}

func init() {
	skillCmd.AddCommand(skillInstallCmd)
	skillCmd.AddCommand(skillUninstallCmd)
	skillCmd.AddCommand(skillUpdateCmd)
	skillCmd.AddCommand(skillStatusCmd)

	skillCmd.PersistentFlags().BoolVar(&skillProject, "project", false, "use project-local .agents/skills/ instead of global ~/.agents/skills/")
}

func runSkillInstall(_ *cobra.Command, _ []string) error {
	skillDir, err := skill.SkillPath(skillProject)
	if err != nil {
		return err
	}

	updated, modified, err := skill.Install(skillDir)
	if err != nil {
		return fmt.Errorf("install skill: %w", err)
	}

	if modified {
		logger.L.Warn("Installed skill has been modified. Run `dargstack skill update` to overwrite with the bundled version.")
		displayDir := skillDir
		if !filepath.IsAbs(skillDir) {
			displayDir = "./" + skillDir
		}
		fmt.Printf("Skill already installed at %s (user-modified)\n", displayDir)
		return nil
	}

	if updated {
		displayDir := skillDir
		if !filepath.IsAbs(skillDir) {
			displayDir = "./" + skillDir
		}
		logger.Success(fmt.Sprintf("Skill installed at %s", displayDir))
		return nil
	}

	displayDir := skillDir
	if !filepath.IsAbs(skillDir) {
		displayDir = "./" + skillDir
	}
	fmt.Printf("Skill already installed at %s (up to date)\n", displayDir)
	return nil
}

func runSkillUninstall(_ *cobra.Command, _ []string) error {
	skillDir, err := skill.SkillPath(skillProject)
	if err != nil {
		return err
	}

	if err := skill.Uninstall(skillDir); err != nil {
		return err
	}

	logger.Success("Skill uninstalled")
	return nil
}

func runSkillUpdate(_ *cobra.Command, _ []string) error {
	skillDir, err := skill.SkillPath(skillProject)
	if err != nil {
		return err
	}

	updated, err := skill.Update(skillDir)
	if err != nil {
		return err
	}

	if updated {
		logger.Success("Skill updated")
	} else {
		fmt.Println("Skill is already up to date")
	}
	return nil
}

func runSkillStatus(_ *cobra.Command, _ []string) error {
	skillDir, err := skill.SkillPath(skillProject)
	if err != nil {
		return err
	}

	info, err := skill.Status(skillDir)
	if err != nil {
		return err
	}

	location := info.Location
	if !filepath.IsAbs(info.Location) {
		location = "./" + location
	}

	trunc := func(h string) string {
		if len(h) > 12 {
			return h[:12] + "..."
		}
		return h
	}

	fmt.Printf("Location:    %s\n", location)
	fmt.Printf("Bundled:     %s (%s)\n", info.BundledVer, trunc(info.BundledHash))

	if !info.Installed {
		fmt.Println("Installed:     no")
		return nil
	}

	fmt.Printf("Installed:   %s (%s)\n", info.InstalledVer, trunc(info.InstalledHash))

	switch {
	case info.UserModified:
		logger.L.Warn("Skill has been modified since installation")
	case info.UpToDate:
		logger.Success("Skill is up to date")
	default:
		logger.L.Info("A new version of the skill is available. Run `dargstack skill update` to update.")
	}

	return nil
}

// autoInstallSkill installs the skill based on the effective skill install mode.
// Called by initialize and clone after project setup.
func autoInstallSkill(projectCfg *config.Config) {
	globalCfg, err := config.LoadGlobalConfig()
	if err != nil {
		logger.L.Warn(fmt.Sprintf("Could not read global config: %v", err))
		return
	}

	mode := config.EffectiveSkillInstall(globalCfg, projectCfg)

	switch mode {
	case config.SkillInstallOff:
		if !noInteraction {
			logger.L.Info("Run `dargstack skill install` to help AI agents understand your stack.")
		}
	case config.SkillInstallOnce:
		skillDir, dirErr := skill.SkillPath(false)
		if dirErr != nil {
			return
		}
		if !skill.IsInstalled(skillDir) {
			updated, modified, installErr := skill.Install(skillDir)
			if installErr != nil {
				logger.L.Warn(fmt.Sprintf("Could not install skill: %v", installErr))
				return
			}
			if updated && !modified {
				logger.L.Info(fmt.Sprintf("Skill installed at %s", skillDir))
			}
		}
	case config.SkillInstallAuto:
		skillDir, dirErr := skill.SkillPath(false)
		if dirErr != nil {
			return
		}
		updated, modified, installErr := skill.Install(skillDir)
		if installErr != nil {
			logger.L.Warn(fmt.Sprintf("Could not install skill: %v", installErr))
			return
		}
		if updated && !modified {
			logger.L.Info(fmt.Sprintf("Skill installed at %s", skillDir))
		} else if modified {
			logger.L.Warn("Installed skill has been modified. Run `dargstack skill update` to overwrite.")
		}
	}
}

// autoUpdateSkill updates the skill after a binary self-update.
// Returns true if the skill was updated, false otherwise.
func autoUpdateSkill() bool {
	globalCfg, err := config.LoadGlobalConfig()
	if err != nil {
		return false
	}
	if globalCfg.Runtime.Skill.Install != config.SkillInstallAuto {
		return false
	}

	skillDir, dirErr := skill.SkillPath(false)
	if dirErr != nil {
		return false
	}

	updated, _, installErr := skill.Install(skillDir)
	if installErr != nil {
		logger.L.Warn(fmt.Sprintf("Could not update skill: %v", installErr))
		return false
	}

	if updated {
		logger.L.Info("Skill updated to match dargstack version")
	}
	return updated
}
