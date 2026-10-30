#!/bin/sh

# Generate Go coverage reports for SonarQube.
# The Go module lives at the repo root, so coverage is generated in place and
# the module prefix is stripped so paths are root-relative (sonar.sources=.).
# Test failures are logged but do not stop coverage generation, so SonarScanner
# can still run and surface test issues in the SonarQube UI.

set -u

REPO_ROOT="$(pwd)"
GO_TEST_FLAGS="${GO_TEST_FLAGS:-}"
SERVER_DIR="${SERVER_DIR:-.}"

mkdir -p .sonar

echo "Generating coverage..."
cd "${SERVER_DIR}" || exit 1
go test ${GO_TEST_FLAGS} -count=1 -coverprofile=coverage.out ./...
server_rc=$?
sed -e 's|^github.com/julienhmmt/helmdownloader/||' coverage.out > "$REPO_ROOT/.sonar/server-coverage.out"

if [ $server_rc -ne 0 ]; then
    echo "Warning: one or more tests failed. Coverage report was still generated." >&2
fi

echo "Coverage reports ready in .sonar/"
