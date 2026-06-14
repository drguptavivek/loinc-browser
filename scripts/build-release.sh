#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="${ROOT_DIR}/dist"
APP_NAME="loinc-browser"
VERSION="${VERSION:-$(tr -d '[:space:]' < "${ROOT_DIR}/VERSION")}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo dev)}"
BUILD_DATE="${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

targets=(
  "darwin/arm64"
  "linux/amd64"
  "windows/amd64"
)

cd "${ROOT_DIR}"

echo "Building web assets..."
npm --prefix web run build

rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}"

for target in "${targets[@]}"; do
  IFS="/" read -r goos goarch goarm <<< "${target}"
  arch_label="${goarch}"
  if [[ -n "${goarm:-}" ]]; then
    arch_label="${goarch}v${goarm}"
  fi
  package_dir="${DIST_DIR}/${APP_NAME}_${VERSION}_${goos}_${arch_label}"
  mkdir -p "${package_dir}"

  binary="${package_dir}/${APP_NAME}"
  if [[ "${goos}" == "windows" ]]; then
    binary="${binary}.exe"
  fi

  echo "Building ${goos}/${arch_label}..."
  CGO_ENABLED=0 GOOS="${goos}" GOARCH="${goarch}" GOARM="${goarm:-}" go build \
    -trimpath \
    -ldflags="-s -w -X loinc-browser/internal/version.Version=${VERSION} -X loinc-browser/internal/version.Commit=${COMMIT} -X loinc-browser/internal/version.Date=${BUILD_DATE}" \
    -o "${binary}" \
    ./cmd/loinc-browser

  cp README.md AGENTS.md ERD.md VERSION CHANGELOG.md .env.example "${package_dir}/"
  cp -R docs skills "${package_dir}/"
  cat > "${package_dir}/INSTALL.md" <<EOF
# ${APP_NAME} ${VERSION}

This package contains only the LOINC Browser application binary and project docs.
Licensed LOINC release files and generated SQLite databases are not included.

## Run

macOS/Linux:

\`\`\`bash
./${APP_NAME}
\`\`\`

Windows PowerShell:

\`\`\`powershell
.\\${APP_NAME}.exe
\`\`\`

Open http://localhost:8080 and upload your licensed LOINC release zip from the loader page. The same command also exposes /api/v1, Swagger/OpenAPI, and HTTP MCP at /mcp.

## License and attribution

LOINC release files and generated databases are not included in this package. LOINC content remains governed by the LOINC Copyright Notice and License:

https://loinc.org/kb/license/

Required LOINC notice:

This material contains content from LOINC (http://loinc.org). LOINC is Copyright © Regenstrief Institute, Inc. and the Logical Observation Identifiers Names and Codes (LOINC) Committee and is available at no cost under the license at http://loinc.org/license. LOINC® is a registered United States trademark of Regenstrief Institute, Inc.

Project documentation and non-LOINC explanatory text may be reused with attribution under CC BY 4.0:

https://creativecommons.org/licenses/by/4.0/

Project source:

https://github.com/drguptavivek/LOINC
EOF

  archive_base="${DIST_DIR}/${APP_NAME}_${VERSION}_${goos}_${arch_label}"
  if [[ "${goos}" == "windows" ]]; then
    (cd "${DIST_DIR}" && zip -qr "${archive_base}.zip" "$(basename "${package_dir}")")
  else
    tar -C "${DIST_DIR}" -czf "${archive_base}.tar.gz" "$(basename "${package_dir}")"
  fi
done

echo "Release packages written to ${DIST_DIR}"
