#!/usr/bin/env bash

set -e

DEBUG="${DEBUG:-false}"
GOCMD="${GOCMD:-go}"
OUTFILE="${OUTFILE:-}"
XUNIT_OUTFILE="${XUNIT_OUTFILE:-}"
JSON_OUTFILE="${JSON_OUTFILE:-}"
COVERAGE_OUTFILE="${COVERAGE_OUTFILE:-}"

function echoDebug {
  if [[ "${DEBUG}" == true ]]; then
    echo "[gotest.sh] $@"
  fi
}

if [[ -n "${OUTFILE}" ]]; then
  mkdir -p "$(dirname "${OUTFILE}")"
else
  OUTFILE="$(mktemp)"
fi
if [[ -n "${XUNIT_OUTFILE}" ]]; then
  mkdir -p "$(dirname "${XUNIT_OUTFILE}")"
fi
if [[ -n "${JSON_OUTFILE}" ]]; then
  mkdir -p "$(dirname "${JSON_OUTFILE}")"
fi
if [[ -n "${COVERAGE_OUTFILE}" ]]; then
  mkdir -p "$(dirname "${COVERAGE_OUTFILE}")"
fi

echoDebug "GOCMD: ${GOCMD}"
echoDebug "Raw output file:      ${OUTFILE}"
echoDebug "JSON output file:     ${JSON_OUTFILE}"
echoDebug "xUnit output file:    ${XUNIT_OUTFILE}"
echoDebug "Coverage output file: ${COVERAGE_OUTFILE}"

exitCodeFile="$(mktemp)"
echo "0" > "${exitCodeFile}"
declare -a modargs
GORACE="-race"
for value in "$@"; do
    if [ "$value" = "-norace" ]; then
        GORACE=""
    elif [ "$value" != "-race" ]; then
        modargs+=("$value")
    fi
done
modargs+=("$GORACE")

if [[ -n "${COVERAGE_OUTFILE}" ]]; then
    echoDebug "Collecting packages for coverage report..."
    coverpkg=""
    for pkg in $(go list ./...); do
        if [[ -n "${coverpkg}" ]]; then
            coverpkg="${coverpkg},"
        fi
        coverpkg="${coverpkg}${pkg}"
    done
    modargs+=("-coverpkg=${coverpkg}")
    modargs+=("-coverprofile=${COVERAGE_OUTFILE}")
fi

if [[ -n "${XUNIT_OUTFILE}" ]]; then
    # jstemmer/go-junit-report requires verbose output
    modargs+=("-v")
fi

echoDebug "Running ${GOCMD} test ${modargs[*]}"
# Disable log coloring (ANSI codes are invalid xml characters)
(2>&1 DEV_DISABLE_LOG_COLORS=true ${GOCMD} test ${modargs[*]} || echo "$?" > "${exitCodeFile}") | tee "${OUTFILE}"
exitCode="$(cat "${exitCodeFile}")"
echoDebug "Tests Exit Code: $exitCode"

if [[ -n "${JSON_OUTFILE}" ]]; then
  echoDebug "Gernerating JSON test report at: ${JSON_OUTFILE}"
  go tool test2json < "${OUTFILE}" > "${JSON_OUTFILE}"
fi

if [[ -n "${XUNIT_OUTFILE}" ]]; then
  echoDebug "Ensuring jstemmer/go-junit-report is installed"
  ${GOCMD} install github.com/jstemmer/go-junit-report@v1.0.0
  echoDebug "Generating xUnit test report at: ${XUNIT_OUTFILE}"
  go-junit-report < "${OUTFILE}" > "${XUNIT_OUTFILE}"
fi

echoDebug "Done"
exit "$exitCode"