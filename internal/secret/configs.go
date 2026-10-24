package secret

// ExtractConfigTemplates extracts x-dargstack.configs from compose data.
// These describe non-secret values derived by dargstack (e.g. a public_key derived from a private_key secret) and materialized as plain Docker configs.
func ExtractConfigTemplates(composeData []byte) (map[string]Template, error) {
	return extractDargstackSection(composeData, "configs", "config")
}

// ExtractConfigPaths extracts file: paths from the top-level configs section of compose data.
func ExtractConfigPaths(composeData []byte) map[string]string {
	return extractResourcePaths(composeData, "configs")
}

// RewriteConfigFilePaths rewrites the configs.NAME.file: entry of every config derived through x-dargstack.configs to point to configsDir/NAME (flat hierarchy), mirroring RewriteSecretFilePaths.
// Plain file-based configs keep their path, since nothing writes them to configsDir.
func RewriteConfigFilePaths(composeData []byte, configsDir string) ([]byte, error) {
	templates, err := ExtractConfigTemplates(composeData)
	if err != nil {
		return nil, err
	}
	if len(templates) == 0 {
		return composeData, nil
	}
	return rewriteResourceFilePaths(composeData, configsDir, "configs", "config", templates)
}

// WriteConfigs writes resolved config values to their compose-declared file paths.
// Unlike WriteSecrets, files are written with standard (world-readable) permissions since configs are not sensitive by definition.
func WriteConfigs(configPaths, values map[string]string) error {
	return writeResourceFiles(configPaths, values, 0o644, "config")
}
