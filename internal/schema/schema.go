package schema

const schemaVersion = "v4"

func SchemaVersion() string { return schemaVersion }

func Schema() string {
	return jsonSchema
}

var jsonSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://dargstack.io/schema/v4/dargstack.json",
  "title": "dargstack.yaml",
  "description": "Configuration file for dargstack - Docker Swarm stack helper CLI",
  "type": "object",
  "properties": {
    "$schema": {
      "type": "string",
      "description": "JSON Schema URI for IDE validation and autocomplete",
      "examples": ["https://dargstack.io/schema/v4/dargstack.json"]
    },
    "metadata": {
      "type": ["object", "null"],
      "description": "Project metadata and CLI compatibility constraints",
      "properties": {
        "name": {
          "type": "string",
          "description": "Stack name (alphanumeric with hyphens/underscores/dots; defaults to parent directory name)"
        },
        "compatibility": {
          "type": "string",
          "description": "Semver range of compatible CLI versions (e.g., \">=4.0.0 <5.0.0\")",
          "examples": [">=4.0.0 <5.0.0"]
        },
        "source": {
          "type": ["object", "null"],
          "description": "Optional source repository metadata",
          "properties": {
            "name": {
              "type": "string",
              "description": "Human-readable source name"
            },
            "url": {
              "type": "string",
              "format": "uri",
              "description": "URL to the source repository"
            }
          },
          "additionalProperties": false
        },
        "external_services": {
          "type": ["object", "null"],
          "description": "External services consumed by the stack",
          "additionalProperties": {
            "type": "object",
            "properties": {
              "description": {
                "type": "string",
                "description": "Human-readable service description"
              }
            },
            "additionalProperties": false
          }
        }
      },
      "additionalProperties": false
    },
    "runtime": {
      "type": ["object", "null"],
      "description": "CLI runtime behavior configuration",
      "properties": {
        "sudo": {
          "type": "string",
          "enum": ["auto", "always", "never"],
          "description": "Whether to use sudo for Docker commands",
          "default": "auto"
        },
        "build": {
          "type": ["object", "null"],
          "properties": {
            "mode": {
              "type": "string",
              "enum": ["always", "missing"],
              "description": "Image build strategy",
              "default": "always"
            }
          },
          "additionalProperties": false
        },
        "deploy": {
          "type": ["object", "null"],
          "properties": {
            "volumes": {
              "type": ["object", "null"],
              "properties": {
                "prompt": {
                  "type": "boolean",
                  "description": "Whether to prompt before mounting volumes",
                  "default": true
                }
              },
              "additionalProperties": false
            }
          },
          "additionalProperties": false
        }
      },
      "additionalProperties": false
    },
    "environment": {
      "type": ["object", "null"],
      "description": "Per-environment configuration",
      "properties": {
        "development": {
          "type": ["object", "null"],
          "properties": {
            "domain": {
              "type": "string",
              "description": "Base domain for the development environment",
              "default": "app.localhost"
            },
            "certificate": {
              "type": ["object", "null"],
              "properties": {
                "include": {
                  "type": "array",
                  "items": { "type": "string" },
                  "description": "Domains to add to the TLS certificate"
                },
                "exclude": {
                  "type": "array",
                  "items": { "type": "string" },
                  "description": "Domains to remove from the TLS certificate"
                }
              },
              "additionalProperties": false
            }
          },
          "additionalProperties": false
        },
        "production": {
          "type": ["object", "null"],
          "properties": {
            "domain": {
              "type": "string",
              "description": "Base domain for the production environment",
              "default": "app.localhost"
            },
            "branch": {
              "type": "string",
              "description": "Git branch to deploy from",
              "default": "main"
            },
            "tag": {
              "type": "string",
              "description": "Git tag to deploy (defaults to auto-detection)"
            }
          },
          "additionalProperties": false
        }
      },
      "additionalProperties": false
    }
  },
  "additionalProperties": false
}`
