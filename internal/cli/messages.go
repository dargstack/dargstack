package cli

// User-facing messages and prompt choices shared across CLI commands.
const (
	// Error prefixes
	ErrFilterComposeByProfile = "filter compose by profile"
	ErrDockerNotRunning       = "docker is not running"
	ErrNoComposeSources       = "no compose sources found"
	ErrValidationFailed       = "validation failed"
	ErrRewriteSecretFilePaths = "rewrite secret file paths"

	// Success messages
	MsgBuiltImage     = "Built %s"
	MsgRemovedVolumes = "Removed %d volume(s)"

	// Info / hint messages
	HintStartDocker        = "Start Docker Desktop or the docker daemon, then try again."
	MsgReplaceSecretFiles  = "Replace those secret files with real values before deploying to production."
	MsgClipboardCopyFailed = "Clipboard copy failed: %v"
	TipAddSecretMetadata   = "Tip: Add x-dargstack.secrets entries with typed secret metadata to auto-generate missing secrets."

	// Flag descriptions
	FlagDescProfiles = "activate one or more compose profiles (or set COMPOSE_PROFILES env var); unlabeled services are included unless a 'default' profile is defined"

	// Prompt choices
	ChoiceAutoGenAll  = "Auto-generate this and remaining auto-generatable secrets"
	ChoiceAutoGenThis = "Auto-generate this secret"
	ChoiceCopyKey     = "Copy key to clipboard"
	ChoiceCopyValue   = "Copy value to clipboard"
)
