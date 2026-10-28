package compose

import (
	"strings"
	"testing"
)

func TestConfigNames(t *testing.T) {
	composeYAML := `configs:
  b-config:
    file: ./b.yaml
  a-config:
    file: ./a.yaml
services:
  api:
    image: api:latest
`
	names, err := ConfigNames([]byte(composeYAML))
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != "a-config" || names[1] != "b-config" {
		t.Errorf("unexpected names: %v", names)
	}

	if names, err = ConfigNames([]byte("services:\n  api:\n    image: api:latest\n")); err != nil {
		t.Fatal(err)
	} else if len(names) != 0 {
		t.Errorf("expected no names, got %v", names)
	}

	if _, err = ConfigNames([]byte("{{invalid")); err == nil {
		t.Error("expected parse error")
	}
}

func TestRenameConfigInComposeText(t *testing.T) {
	const input = `# top-level comment
services:
  api:
    image: api:latest
    configs: [prom-config-old, other-config] # trailing comment
    environment:
      - CONFIG_NAME=prom-config-old
    deploy:
      labels:
        - prom-config-old.label=keep  # dargstack:dev-only
  verifier:
    image: verifier:latest
    configs:
      - prom-config-old
      - source: prom-config-old
        target: /etc/p.conf
configs:
  prom-config-old:  # the config itself
    file: ./prom.yaml
  prom-config-old-long:
    file: ./long.yaml
x-dargstack:
  configs:
    prom-config-old:
      type: public_key
      source: prom-config-old
`

	const want = `# top-level comment
services:
  api:
    image: api:latest
    configs: [prom-config-new, other-config] # trailing comment
    environment:
      - CONFIG_NAME=prom-config-old
    deploy:
      labels:
        - prom-config-old.label=keep  # dargstack:dev-only
  verifier:
    image: verifier:latest
    configs:
      - prom-config-new
      - source: prom-config-new
        target: /etc/p.conf
configs:
  prom-config-new:  # the config itself
    file: ./prom.yaml
  prom-config-old-long:
    file: ./long.yaml
x-dargstack:
  configs:
    prom-config-old:
      type: public_key
      source: prom-config-old
`

	got, changed := RenameConfigInComposeText(input, "prom-config-old", "prom-config-new")
	if !changed {
		t.Fatal("expected changes")
	}
	if got != want {
		t.Errorf("unexpected output:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenameConfigInComposeTextNoMatch(t *testing.T) {
	const input = `services:
  api:
    image: api:latest
    configs:
      - other
configs:
  other:
    file: ./other.yaml
`
	got, changed := RenameConfigInComposeText(input, "absent", "absent-new")
	if changed {
		t.Errorf("expected no changes:\n%s", got)
	}
	if got != input {
		t.Errorf("text must be byte-identical when nothing matched")
	}
}

func TestRenameConfigInComposeTextQuotedItem(t *testing.T) {
	const input = `services:
  api:
    image: api:latest
    configs:
      - "prom-config"
configs:
  prom-config:
    file: ./prom.yaml
`
	got, changed := RenameConfigInComposeText(input, "prom-config", "prom-config-a1b2c3d4")
	if !changed {
		t.Fatal("expected changes")
	}
	if !strings.Contains(got, `- "prom-config-a1b2c3d4"`) {
		t.Errorf("expected quoted item renamed in:\n%s", got)
	}
	if !strings.Contains(got, "  prom-config-a1b2c3d4:") {
		t.Errorf("expected top-level key renamed in:\n%s", got)
	}
}

func TestRenameConfigInComposeTextServiceWithSameName(t *testing.T) {
	// A service that shares the config's name must not be renamed; only the config key and references are.
	const input = `services:
  app-config:
    image: app:latest
    configs:
      - app-config
configs:
  app-config:
    file: ./app.yaml
`
	got, changed := RenameConfigInComposeText(input, "app-config", "app-config-12345678")
	if !changed {
		t.Fatal("expected changes")
	}
	want := `services:
  app-config:
    image: app:latest
    configs:
      - app-config-12345678
configs:
  app-config-12345678:
    file: ./app.yaml
`
	if got != want {
		t.Errorf("unexpected output:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenameConfigInComposeTextLongFormItems(t *testing.T) {
	// The child keys of a long-form item (target, mode) must not hide the following list items.
	const input = `configs:
  alpha:
    file: ./alpha.conf
  beta:
    file: ./beta.conf
services:
  demo:
    image: demo:latest
    configs:
      - source: alpha
        target: /etc/alpha.conf
        mode: "0444"
      - source: beta
        target: /etc/beta.conf
        mode: "0444"
`
	got, changed := RenameConfigInComposeText(input, "beta", "beta-b2c3d4e5")
	if !changed {
		t.Fatal("expected changes")
	}
	want := `configs:
  alpha:
    file: ./alpha.conf
  beta-b2c3d4e5:
    file: ./beta.conf
services:
  demo:
    image: demo:latest
    configs:
      - source: alpha
        target: /etc/alpha.conf
        mode: "0444"
      - source: beta-b2c3d4e5
        target: /etc/beta.conf
        mode: "0444"
`
	if got != want {
		t.Errorf("unexpected output:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenameConfigInComposeTextSameIndentItems(t *testing.T) {
	// List items may sit at the same indent as the configs key; the configs frame must survive the pop.
	const input = `services:
  demo:
    image: demo:latest
    configs:
    - alpha
    - source: beta
      target: /etc/beta.conf
configs:
  alpha:
    file: ./alpha.conf
  beta:
    file: ./beta.conf
`
	got, changed := RenameConfigInComposeText(input, "beta", "beta-b2c3d4e5")
	if !changed {
		t.Fatal("expected changes")
	}
	want := `services:
  demo:
    image: demo:latest
    configs:
    - alpha
    - source: beta-b2c3d4e5
      target: /etc/beta.conf
configs:
  alpha:
    file: ./alpha.conf
  beta-b2c3d4e5:
    file: ./beta.conf
`
	if got != want {
		t.Errorf("unexpected output:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenameConfigInComposeTextSharedDefinition(t *testing.T) {
	// Services inherit the configs list through a YAML anchor and merge key; the renamer rewrites the shared
	// definition's list, the only place the reference text exists.
	const input = `x-shared:
  base:
    &base
    image: example/app:1.0.0
    configs:
      - source: app-config
        target: /etc/app/config.yaml
        mode: "0444"
      - source: app-config-two
        target: /etc/app/config2.yaml
        mode: "0444"
services:
  one:
    <<: *base
    command: ["one"]
  two:
    <<: *base
    command: ["two"]
configs:
  app-config:
    file: ./config.yaml
  app-config-two:
    file: ./config2.yaml
`
	got, changed := RenameConfigInComposeText(input, "app-config-two", "app-config-two-b2c3d4e5")
	if !changed {
		t.Fatal("expected changes")
	}
	want := `x-shared:
  base:
    &base
    image: example/app:1.0.0
    configs:
      - source: app-config
        target: /etc/app/config.yaml
        mode: "0444"
      - source: app-config-two-b2c3d4e5
        target: /etc/app/config2.yaml
        mode: "0444"
services:
  one:
    <<: *base
    command: ["one"]
  two:
    <<: *base
    command: ["two"]
configs:
  app-config:
    file: ./config.yaml
  app-config-two-b2c3d4e5:
    file: ./config2.yaml
`
	if got != want {
		t.Errorf("unexpected output:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenameConfigInComposeTextSharedFlowList(t *testing.T) {
	const input = `x-shared:
  base:
    &base
    image: example/app:1.0.0
    configs: [app-config, other-config]
services:
  one:
    <<: *base
configs:
  app-config:
    file: ./config.yaml
`
	got, changed := RenameConfigInComposeText(input, "app-config", "app-config-b2c3d4e5")
	if !changed {
		t.Fatal("expected changes")
	}
	want := `x-shared:
  base:
    &base
    image: example/app:1.0.0
    configs: [app-config-b2c3d4e5, other-config]
services:
  one:
    <<: *base
configs:
  app-config-b2c3d4e5:
    file: ./config.yaml
`
	if got != want {
		t.Errorf("unexpected output:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenameConfigInComposeTextBlockScalarContentNotRenamed(t *testing.T) {
	// Block scalar payloads may contain lines that look like config references; they are data and must
	// survive rotation untouched, while real references outside the scalars are still renamed.
	const input = `x-payloads:
  doc:
    configs: |
      - app-config
      - other
  template: &tpl >
    configs: [app-config]
    note: keep this
services:
  demo:
    image: demo:latest
    configs:
      - source: app-config
        target: /etc/app/config.yaml
configs:
  app-config:
    file: ./app.yaml
`
	got, changed := RenameConfigInComposeText(input, "app-config", "app-config-b2c3d4e5")
	if !changed {
		t.Fatal("expected changes")
	}
	want := `x-payloads:
  doc:
    configs: |
      - app-config
      - other
  template: &tpl >
    configs: [app-config]
    note: keep this
services:
  demo:
    image: demo:latest
    configs:
      - source: app-config-b2c3d4e5
        target: /etc/app/config.yaml
configs:
  app-config-b2c3d4e5:
    file: ./app.yaml
`
	if got != want {
		t.Errorf("unexpected output:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenameConfigInComposeTextDeepNestedNoMatch(t *testing.T) {
	// A configs list deeper than a direct child of a top-level section is not a rename site.
	const input = `x-shared:
  base:
    nested:
      configs:
        - app-config
configs:
  app-config:
    file: ./config.yaml
`
	got, changed := RenameConfigInComposeText(input, "app-config", "app-config-b2c3d4e5")
	if !changed {
		t.Fatal("expected the top-level key to be renamed")
	}
	if strings.Contains(got, "- app-config-b2c3d4e5") {
		t.Errorf("deep nested reference must not be renamed:\n%s", got)
	}
	if !strings.Contains(got, "  app-config-b2c3d4e5:") {
		t.Errorf("expected top-level key renamed in:\n%s", got)
	}
}

func TestRenameConfigInComposeTextFlowListNoMatch(t *testing.T) {
	const input = `services:
  api:
    image: api:latest
    configs: [other-config]
`
	got, changed := RenameConfigInComposeText(input, "config", "config-12345678")
	if changed {
		t.Errorf("substring of another name must not be renamed:\n%s", got)
	}
	if got != input {
		t.Errorf("text must be unchanged")
	}
}
