#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
sdk_root="$(cd "${script_dir}/.." && pwd)"
schema_path="${sdk_root}/codex-go/internal/protocol/schema/v2.schema.json"
version_path="${sdk_root}/codex-go/internal/protocol/schema/version.go"

python3 - <<'PY' "${schema_path}" "${version_path}"
import hashlib
import json
import pathlib
import sys

schema_path = pathlib.Path(sys.argv[1])
version_path = pathlib.Path(sys.argv[2])
data = json.loads(schema_path.read_text())
digest = hashlib.sha256(schema_path.read_bytes()).hexdigest()
definition_count = len(data.get("$defs", {}))
version_path.write_text(
    "package schema\n\n"
    "// Schema metadata is regenerated from v2.schema.json by the SDK sync script.\n"
    "const (\n"
    f'\tSchemaTitle = {json.dumps(data["title"])}\n'
    f'\tSchemaRevision = {json.dumps(digest)}\n'
    f"\tSchemaDefinitionCount = {definition_count}\n"
    ")\n"
)
PY
