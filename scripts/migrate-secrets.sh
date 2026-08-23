#!/usr/bin/env bash
#
# Migrates a Docker Swarm stack's secrets from their dargstack v3 names to the dargstack v4 names.
# v4 defines secrets with a flat kebab-case name (e.g. `my-stack-api-key`) while v3 included underscores (e.g. `my-stack_api-key`).
# Every secret therefore already exists in the swarm under its old, underscore-containing name and just needs to be re-created under its new, pure-kebab-case name with the same value.
# The set of secrets to migrate is derived from the swarm's current secret list, not hardcoded (see `PAIRS` below).
#
# Run this on a swarm manager node before deploying the v4 branch to production.
# It starts one throwaway swarm service that mounts every old secret, dumps each value to a bind-mounted temporary directory, then creates the new secrets from those dumped files.
# It only creates new secrets, it never deletes or modifies the old ones.
# Old secrets are left in place to be removed manually once the v4 stack is confirmed healthy.
#
# If a secret's v4 name isn't simply its v3 name with underscores replaced by dashes (e.g. it was renamed for other reasons during your v3 history), add an entry to OVERRIDES below in the form "old_name:new_name".

set -euo pipefail

if [ "$(docker info --format '{{.Swarm.ControlAvailable}}' 2>/dev/null)" != "true" ]; then
  echo "error: this must run on a swarm manager node" >&2
  exit 1
fi

# Format: "old_name:new_name"
OVERRIDES=()

override_for() {
  local old_name="$1" pair
  for pair in "${OVERRIDES[@]:-}"; do
    [ -z "$pair" ] && continue
    if [ "${pair%%:*}" = "$old_name" ]; then
      echo "${pair##*:}"
      return 0
    fi
  done
  return 1
}

PAIRS=()
while IFS= read -r old_name; do
  [ -z "$old_name" ] && continue
  if new_name="$(override_for "$old_name")"; then
    :
  else
    new_name="${old_name//_/-}"
  fi
  PAIRS+=("$old_name:$new_name")
done < <(docker secret ls --format '{{.Name}}' | grep '_' || true)

DUMP_DIR="$(mktemp -d)"
trap 'rm -rf "$DUMP_DIR"' EXIT

created=0
skipped=0
missing=0

# Collect the pairs whose old secret exists and new secret doesn't yet, so only what's actually needed gets mounted and already-migrated ones are skipped up front.
to_migrate=()
for pair in "${PAIRS[@]}"; do
  old_name="${pair%%:*}"
  new_name="${pair##*:}"

  if docker secret inspect "$new_name" >/dev/null 2>&1; then
    echo "skip    $new_name (already exists)"
    skipped=$((skipped + 1))
    continue
  fi

  if ! docker secret inspect "$old_name" >/dev/null 2>&1; then
    echo "MISSING $new_name <- swarm secret '$old_name' not found, needs manual handling"
    missing=$((missing + 1))
    continue
  fi

  to_migrate+=("$pair")
done

if [ "${#to_migrate[@]}" -eq 0 ]; then
  echo
  echo "done: nothing to migrate ($skipped already existed, $missing missing)"
  exit 0
fi

secret_args=()
copy_cmds=()
for pair in "${to_migrate[@]}"; do
  old_name="${pair%%:*}"
  secret_args+=(--secret "source=$old_name,target=$old_name")
  copy_cmds+=("cp /run/secrets/$old_name /dump/$old_name")
done

docker service create \
  --name secret-migrate-dump \
  --quiet \
  --detach \
  --restart-condition none \
  "${secret_args[@]}" \
  --mount type=bind,source="$DUMP_DIR",target=/dump \
  alpine:3 sh -c "$(printf '%s && ' "${copy_cmds[@]}")true" >/dev/null

for _ in $(seq 1 60); do
  state="$(docker service ps --format '{{.CurrentState}}' secret-migrate-dump 2>/dev/null | head -1)"
  case "$state" in
    Complete*) break ;;
    Failed*)
      echo "error: dump task failed" >&2
      docker service logs secret-migrate-dump >&2 || true
      docker service rm secret-migrate-dump >/dev/null
      exit 1
      ;;
  esac
  sleep 1
done
docker service rm secret-migrate-dump >/dev/null

for pair in "${to_migrate[@]}"; do
  old_name="${pair%%:*}"
  new_name="${pair##*:}"

  docker secret create "$new_name" "$DUMP_DIR/$old_name" >/dev/null
  echo "created $new_name <- $old_name"
  created=$((created + 1))
done

find "$DUMP_DIR" -type f -exec shred -u {} \; 2>/dev/null || rm -rf "${DUMP_DIR:?}"/*

echo
echo "done: $created created, $skipped already existed, $missing missing (see MISSING lines above)"
echo "old secrets were left untouched; remove them manually once the v4 stack is confirmed healthy"
