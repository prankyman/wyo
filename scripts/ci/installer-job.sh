#!/usr/bin/env bash
# The pipeline's installer job (.github/workflows/ci-cd.yml, job "installer"): proves on a clean machine with systemd that
# the installer and the updater of THIS commit work, against a test certificate authority (Pebble) and a local server
# of release files signed with a key made for the run. It never calls the real Let's Encrypt, the real release address
# or any marketplace.
#
#   scripts/ci/installer-job.sh build   (as the runner's user) builds the programs and the files of the releases
#   sudo -E scripts/ci/installer-job.sh run   (as root) runs the installer and checks everything
#
# The test copy of a release is made here: the wyo server compiled with the Go build tag wyotest, and install.sh with its
# two constants (the release address and the public key) replaced by the run's own. The released installer and the
# released wyo server have no setting that changes either.
set -euo pipefail

PEBBLE_VERSION=v2.10.3
PEBBLE_SHA256_AMD64=7e33cf3e426b67e9e647c44e8cc91683caa8ab331f7a99e5e27fcee291c4c7e2
PEBBLE_SHA256_ARM64=1dc8fa9b1dd68529eb62fe685d3a40d5929cea8a17fefbff0940aed0901a1a47

DOMAIN=wyo-test.example.org
OWNER_LOGIN=owner-of-the-job
RELEASE_PORT=8099
VERSION_FIRST=1.0.0
VERSION_NEWER=1.1.0
VERSION_BROKEN=1.2.0
VERSION_CHANGED=1.3.0
VERSION_PACKAGE=github.com/prankyman/wyo/internal/version.Version

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
DIR=${INSTALLER_JOB_DIR:-${RUNNER_TEMP:-/tmp}/installer-job}

case $(uname -m) in
    x86_64) ARCH=amd64 ;;
    aarch64) ARCH=arm64 ;;
    *) echo "unsupported processor $(uname -m)" >&2 && exit 1 ;;
esac

step() { printf '\n==> %s\n' "$*"; }
passed() { printf 'PASS: %s\n' "$*"; }
fail() {
    printf 'FAIL: %s\n' "$*" >&2
    exit 1
}

# expect DESCRIPTION COMMAND…: the command must succeed.
expect() {
    local description=$1
    shift
    "$@" >/dev/null 2>&1 || fail "$description"
    passed "$description"
}

# expect_equal DESCRIPTION WANT GOT
expect_equal() {
    [ "$2" = "$3" ] || fail "$1 (wanted '$2', got '$3')"
    passed "$1"
}

# wait_until DESCRIPTION SECONDS COMMAND…: asks every two seconds until the command succeeds.
wait_until() {
    local description=$1 seconds=$2 spent=0
    shift 2
    until "$@" >/dev/null 2>&1; do
        [ "$spent" -lt "$seconds" ] || fail "no answer in $seconds seconds: $description"
        sleep 2
        spent=$((spent + 2))
    done
    passed "$description"
}

summary() {
    if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then printf '%s\n' "$*" >>"$GITHUB_STEP_SUMMARY"; fi
}

json_field() { # FILE KEY: the value of a top-level key of a JSON file, or nothing
    python3 -I -c 'import json,sys; v=json.load(open(sys.argv[1])).get(sys.argv[2]); print("" if v is None else v)' "$1" "$2"
}

version_of() { "$1" version | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -n 1; }

# ---------------------------------------------------------------------------------------------------------------------
# build: as the runner's user
# ---------------------------------------------------------------------------------------------------------------------
build_programs() {
    step "Build the test programs for $ARCH"
    mkdir -p "$DIR/programs"
    cd "$ROOT"
    local version
    for version in "$VERSION_FIRST" "$VERSION_NEWER" "$VERSION_CHANGED"; do
        CGO_ENABLED=0 go build -trimpath -tags wyotest -ldflags "-s -w -X $VERSION_PACKAGE=$version" \
            -o "$DIR/programs/wyo-$version" ./cmd/wyo
    done
    # A "new version" that never answers: it exits at once, so the service never comes up.
    printf '#!/bin/sh\nexit 1\n' >"$DIR/programs/wyo-$VERSION_BROKEN"
    chmod 0755 "$DIR/programs/wyo-$VERSION_BROKEN"

    # The size of the program a seller downloads: the real release build, without the test tag.
    CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X $VERSION_PACKAGE=0.0.0" -o "$DIR/programs/wyo-release-size" ./cmd/wyo
    stat -c %s "$DIR/programs/wyo-release-size" >"$DIR/release-size-bytes"
    rm -f "$DIR/programs/wyo-release-size"
    passed "programs built"
}

# ---------------------------------------------------------------------------------------------------------------------
# run: as root
# ---------------------------------------------------------------------------------------------------------------------
JAR=$DIR/cookies.txt
SERVER_LOG=$DIR/release-server.log
PEBBLE_LOG=$DIR/pebble.log

DUMPED=0
dump_logs() {
    if [ "$DUMPED" = 1 ]; then return 0; fi
    DUMPED=1
    echo "---- the job failed; the logs follow ----" >&2
    journalctl -u wyo.service -u wyo-update.service --no-pager -n 120 >&2 || true
    tail -n 60 "$PEBBLE_LOG" >&2 || true
    tail -n 40 "$SERVER_LOG" >&2 || true
}

# Whatever ends the run, a failed run prints the logs first, and the helpers it started are stopped.
on_exit() {
    local code=$?
    if [ "$code" != 0 ]; then dump_logs; fi
    local pid
    for pid in "${PEBBLE_PID:-}" "${SERVER_PID:-}"; do
        if [ -n "$pid" ]; then kill "$pid" 2>/dev/null || true; fi
    done
    exit "$code"
}

prepare_machine() {
    step "Prepare the machine"
    systemctl stop apache2 nginx 2>/dev/null || true
    for port in 80 443; do
        [ -z "$(ss -H -ltn "sport = :$port")" ] || fail "port $port is taken before the job started"
    done
    printf '127.0.0.1 %s\n' "$DOMAIN" >>/etc/hosts
    passed "the domain $DOMAIN points to this machine"
}

start_pebble() {
    step "Start Pebble $PEBBLE_VERSION, the test certificate authority"
    local folder=$DIR/pebble sum=$PEBBLE_SHA256_AMD64
    mkdir -p "$folder"
    if [ "$ARCH" = arm64 ]; then sum=$PEBBLE_SHA256_ARM64; fi
    curl -fsSL -o "$folder/pebble.tar.gz" \
        "https://github.com/letsencrypt/pebble/releases/download/$PEBBLE_VERSION/pebble-linux-$ARCH.tar.gz"
    echo "$sum  $folder/pebble.tar.gz" | sha256sum -c - >/dev/null || fail "the checksum of Pebble's archive is not the pinned one"
    tar -xzf "$folder/pebble.tar.gz" -C "$folder"
    chmod 0755 "$folder/pebble-linux-$ARCH/linux/$ARCH/pebble"

    # The certificate Pebble's own address answers with, for the wyo server to trust (WYO_ACME_ROOTS).
    openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 -nodes -days 2 -subj /CN=pebble \
        -keyout "$folder/key.pem" -out "$folder/cert.pem" \
        -addext "subjectAltName=DNS:localhost,IP:127.0.0.1" -addext "basicConstraints=critical,CA:TRUE" 2>/dev/null
    cat >"$folder/config.json" <<JSON
{
  "pebble": {
    "listenAddress": "127.0.0.1:14000",
    "managementListenAddress": "127.0.0.1:15000",
    "certificate": "$folder/cert.pem",
    "privateKey": "$folder/key.pem",
    "httpPort": 80,
    "tlsPort": 443,
    "ocspResponderURL": "",
    "externalAccountBindingRequired": false
  }
}
JSON
    # The challenge is really answered by the wyo server on port 80: Pebble is not told to accept it blindly.
    PEBBLE_VA_NOSLEEP=1 PEBBLE_WFE_NONCEREJECT=0 nohup "$folder/pebble-linux-$ARCH/linux/$ARCH/pebble" \
        -config "$folder/config.json" >"$PEBBLE_LOG" 2>&1 &
    PEBBLE_PID=$!
    wait_until "Pebble answers" 30 curl -fsS --cacert "$folder/cert.pem" https://127.0.0.1:14000/dir
    # The root that signs what Pebble issues: curl trusts it, through the variable curl reads, in every call below.
    curl -fsS --cacert "$folder/cert.pem" https://127.0.0.1:15000/roots/0 >"$DIR/pebble-root.pem"
    export CURL_CA_BUNDLE=$DIR/pebble-root.pem
    install -d -m 0755 /etc/wyo-test
    install -m 0644 "$folder/cert.pem" /etc/wyo-test/pebble-ca.pem
}

# make_release NAME VERSION PROGRAM [CHANGED_PROGRAM]: the files of a release, signed with the run's key. When
# CHANGED_PROGRAM is given, the program is replaced by it AFTER the list was signed.
make_release() {
    local folder=$DIR/releases/$1 version=$2 program=$3 changed=${4:-}
    mkdir -p "$folder"
    printf '%s\n' "$version" >"$folder/VERSION"
    cp "$program" "$folder/wyo_linux_amd64"
    cp "$program" "$folder/wyo_linux_arm64"
    cp "$DIR/install-test.sh" "$folder/install.sh"
    (cd "$folder" && sha256sum VERSION wyo_linux_amd64 wyo_linux_arm64 install.sh >checksums.txt &&
        openssl pkeyutl -sign -inkey "$DIR/release-key.pem" -rawin -in checksums.txt -out checksums.txt.sig)
    if [ -n "$changed" ]; then
        cp "$changed" "$folder/wyo_linux_amd64"
        cp "$changed" "$folder/wyo_linux_arm64"
    fi
}

serve_release() { ln -sfn "$DIR/releases/$1" "$DIR/served"; }

prepare_releases() {
    step "Make the run's key, the test copy of the installer and the releases"
    openssl genpkey -algorithm ed25519 -out "$DIR/release-key.pem" 2>/dev/null
    openssl pkey -in "$DIR/release-key.pem" -pubout -out "$DIR/release-public.pem"
    install -m 0644 "$DIR/release-public.pem" /etc/wyo-test/release-public.pem
    local public_body
    public_body=$(sed -n 2p "$DIR/release-public.pem")

    sed -e "s|^RELEASE_URL=.*|RELEASE_URL='http://127.0.0.1:$RELEASE_PORT'|" \
        -e "s|^PUBLIC_KEY_B64=.*|PUBLIC_KEY_B64='$public_body'|" "$ROOT/scripts/install.sh" >"$DIR/install-test.sh"
    grep -qF "RELEASE_URL='http://127.0.0.1:$RELEASE_PORT'" "$DIR/install-test.sh" || fail "the release address was not replaced"
    grep -qF "PUBLIC_KEY_B64='$public_body'" "$DIR/install-test.sh" || fail "the public key was not replaced"
    passed "the test copy of the installer carries the run's address and key"

    local programs=$DIR/programs
    make_release first "$VERSION_FIRST" "$programs/wyo-$VERSION_FIRST"
    make_release newer "$VERSION_NEWER" "$programs/wyo-$VERSION_NEWER"
    make_release broken "$VERSION_BROKEN" "$programs/wyo-$VERSION_BROKEN"
    make_release changed "$VERSION_CHANGED" "$programs/wyo-$VERSION_CHANGED" "$programs/wyo-$VERSION_FIRST"
    serve_release first
    nohup python3 -I -m http.server "$RELEASE_PORT" --bind 127.0.0.1 --directory "$DIR/served" >"$SERVER_LOG" 2>&1 &
    SERVER_PID=$!
    wait_until "the release server answers" 20 curl -fsS "http://127.0.0.1:$RELEASE_PORT/VERSION"
}

# What a test build reads (Plan 2, section A), given to the units by a drop-in the released installer never writes. The
# mode release makes the test build behave as an installed wyo server: it listens on ports 80 and 443 for the domain.
write_test_environment() {
    step "Give the units the test authority, the release server and the run's key"
    local unit
    for unit in wyo.service wyo-update.service; do
        install -d -m 0755 "/etc/systemd/system/$unit.d"
        cat >"/etc/systemd/system/$unit.d/test.conf" <<CONF
[Service]
Environment=WYO_MODE=release
Environment=WYO_ACME_CA=https://127.0.0.1:14000/dir
Environment=WYO_ACME_ROOTS=/etc/wyo-test/pebble-ca.pem
Environment=WYO_RELEASE_URL=http://127.0.0.1:$RELEASE_PORT/
Environment=WYO_RELEASE_TEST_KEY=/etc/wyo-test/release-public.pem
CONF
    done
}

sign_in() { # PASSWORD: the HTTP status of the sign-in call
    curl -s -o /dev/null -w '%{http_code}' -c "$JAR" -H 'Content-Type: application/json' \
        --data "{\"login\":\"$OWNER_LOGIN\",\"password\":\"$1\"}" "https://$DOMAIN/api/v1/session"
}

healthy() { [ "$(curl -fsS --max-time 5 "https://$DOMAIN/healthz")" = ok ]; }

main_pid() { systemctl show wyo.service -p MainPID --value; }

state_field() { json_field /var/lib/wyo/update-state.json "$1"; }

state_is() { [ "$(state_field "$1")" = "$2" ]; }

snapshot() { # what a second run of the installer must leave as it is
    {
        sha256sum /usr/local/bin/wyo /etc/wyo/wyo.env /etc/systemd/system/wyo.service /etc/systemd/system/wyo-update.service \
            /etc/systemd/system/wyo-update.timer /etc/systemd/system/wyo-update.path /var/lib/wyo/secret.key
        find /var/lib/wyo/certs -type f -exec sha256sum {} + | sort
        stat -c '%a %U:%G %n' /usr/local/bin/wyo /etc/wyo/wyo.env /var/lib/wyo /var/lib/wyo/secret.key
    }
}

# The timer's schedule as systemd read it: "{ OnCalendar=*-*-* 01:00:00 Europe/Moscow ; next_elapse=… }".
timer_is_moscow() { systemctl show -p TimersCalendar --value wyo-update.timer | grep -qF '01:00:00 Europe/Moscow'; }

# The unit file of the daily look holds the text the installed program prints.
timer_is_the_programs() { cmp -s <(/usr/local/bin/wyo update --timer-unit) /etc/systemd/system/wyo-update.timer; }

# write_earlier_timer puts back the timer an installer of version 0.0.3 wrote: once a day at a random moment.
write_earlier_timer() {
    printf '%s\n' '[Unit]' 'Description=wyo daily look for a newer version' '' '[Timer]' 'OnCalendar=daily' \
        'RandomizedDelaySec=24h' 'Persistent=true' '' '[Install]' 'WantedBy=timers.target' >/etc/systemd/system/wyo-update.timer
    systemctl daemon-reload
    systemctl restart wyo-update.timer
    if timer_is_moscow; then fail "the earlier timer was not read"; fi
}

check_installation() {
    step "The installation: user, modes, certificate, sign-in"
    expect "the service is active" systemctl is-active --quiet wyo.service
    expect_equal "the wyo server runs as the user wyo" wyo "$(ps -o user= -p "$(main_pid)" | tr -d ' ')"
    expect_equal "the program: mode and owner" "755 root:root" "$(stat -c '%a %U:%G' /usr/local/bin/wyo)"
    expect_equal "wyo.env: mode and owner" "640 root:wyo" "$(stat -c '%a %U:%G' /etc/wyo/wyo.env)"
    expect_equal "the data folder: mode and owner" "750 wyo:wyo" "$(stat -c '%a %U:%G' /var/lib/wyo)"
    expect_equal "secret.key: mode and owner" "600 wyo:wyo" "$(stat -c '%a %U:%G' /var/lib/wyo/secret.key)"
    expect_equal "secret.key holds 32 bytes" 32 "$(stat -c %s /var/lib/wyo/secret.key)"
    expect_equal "certs/: mode and owner" "700 wyo:wyo" "$(stat -c '%a %U:%G' /var/lib/wyo/certs)"
    expect_equal "backups/: mode and owner" "700 wyo:wyo" "$(stat -c '%a %U:%G' /var/lib/wyo/backups)"
    expect "the database exists and belongs to wyo" test "$(stat -c %U /var/lib/wyo/wyo.db)" = wyo
    expect "the daily look runs at 01:00 Moscow time" timer_is_moscow
    expect_equal "the daily look is moved by up to two hours" 2h "$(systemctl show -p RandomizedDelayUSec --value wyo-update.timer)"
    expect_equal "the move is fixed for this server" yes "$(systemctl show -p FixedRandomDelay --value wyo-update.timer)"

    wait_until "https answers" 120 healthy
    expect "the certificate is the test authority's" \
        bash -c "echo | openssl s_client -connect 127.0.0.1:443 -servername $DOMAIN 2>/dev/null | openssl x509 -noout -issuer | grep -qi pebble"
    expect_equal "http:// is sent to https://" "308 https://$DOMAIN/orders?x=1" \
        "$(curl -s -o /dev/null -w '%{http_code} %{redirect_url}' "http://$DOMAIN/orders?x=1")"

    expect_equal "a wrong password is refused" 401 "$(sign_in 'not-the-password-0')"
    expect_equal "the owner signs in" 200 "$(sign_in "$OWNER_PASSWORD")"
    expect_equal "the session is recognised" 200 "$(curl -s -o /dev/null -w '%{http_code}' -b "$JAR" "https://$DOMAIN/api/v1/session")"
}

measure() {
    step "Measure the wyo server (section J of the plan)"
    local pid before after peak
    pid=$(main_pid)
    before=$(awk '/^VmRSS:/ {print $2}' "/proc/$pid/status")
    # 500 calls of the start page and 500 of the API, on connections curl keeps open.
    local urls=() count=0
    while [ "$count" -lt 500 ]; do
        # curl writes each address's answer by the -o given before it: every one goes to /dev/null, not into the log.
        urls+=(-o /dev/null "https://$DOMAIN/" -o /dev/null "https://$DOMAIN/api/v1/meta")
        count=$((count + 1))
    done
    curl -s -b "$JAR" "${urls[@]}"
    after=$(awk '/^VmRSS:/ {print $2}' "/proc/$pid/status")
    peak=$(awk '/^VmHWM:/ {print $2}' "/proc/$pid/status")
    local size
    size=$(cat "$DIR/release-size-bytes")
    printf 'memory after start: %s kB; after 1000 requests: %s kB; peak: %s kB; release program: %s bytes\n' \
        "$before" "$after" "$peak" "$size"
    summary "### Measured on $(uname -m), $(grep -m1 '^PRETTY_NAME=' /etc/os-release | cut -d'"' -f2)"
    summary ""
    summary "| What | Value |"
    summary "| --- | --- |"
    summary "| Memory of the wyo server after its start | $((before / 1024)) MB ($before kB) |"
    summary "| Memory after 1 000 page and API requests | $((after / 1024)) MB ($after kB) |"
    summary "| Peak memory | $((peak / 1024)) MB ($peak kB) |"
    summary "| Size of the released program file | $((size / 1024 / 1024)) MB ($size bytes) |"
    summary ""
}

second_run_changes_nothing() {
    step "A second run of the installer changes nothing"
    local before after pid
    before=$(snapshot)
    pid=$(main_pid)
    bash "$DIR/install-test.sh" </dev/null >"$DIR/second-run.log" 2>&1 || { cat "$DIR/second-run.log" >&2; fail "the second run of the installer failed"; }
    after=$(snapshot)
    [ "$before" = "$after" ] || { diff <(echo "$before") <(echo "$after") >&2 || true; fail "the second run changed files"; }
    passed "files, modes, units, key file and certificate are as they were"
    expect_equal "the service was not restarted" "$pid" "$(main_pid)"
    expect_equal "the owner still signs in" 200 "$(sign_in "$OWNER_PASSWORD")"
}

request_update() { # asks, as the owner, for an installation: the interface's call, the request file, the path unit
    curl -fsS -o /dev/null -b "$JAR" -X POST -H 'X-Wyo-Person: 1' "https://$DOMAIN/api/v1/updates/install"
}

# The updater has ended its run: a one-shot unit is "activating" while it runs and "inactive" once it is done.
updater_finished() { [ "$(systemctl show -p ActiveState --value wyo-update.service)" = inactive ]; }

installed_version_is() { [ "$(version_of /usr/local/bin/wyo)" = "$1" ] && systemctl is-active --quiet wyo.service && healthy; }

# leftover_work_folders: the updater's work folders still in the data folder, one per line.
leftover_work_folders() { find /var/lib/wyo -mindepth 1 -maxdepth 1 -type d -name 'update-*'; }

# The installer, run again with a newer release on a server where wyo runs, does not replace the program itself: it asks
# the updater, as the owner's press does. That the updater did it shows in the copy of the database it makes before an
# update (the installer makes none) and in its state file. The owner's press itself is proven by the broken release.
rerun_installs_newer_through_updater() {
    step "A second run of the installer with a newer release goes through the updater and the data stays"
    local key_before
    key_before=$(sha256sum /var/lib/wyo/secret.key)
    serve_release newer
    bash "$DIR/install-test.sh" </dev/null >"$DIR/rerun-newer.log" 2>&1 || { cat "$DIR/rerun-newer.log" >&2; fail "the run of the installer with a newer release failed"; }
    expect "the installer says the updater installed the version" grep -qF "wyo обновлён до версии $VERSION_NEWER" "$DIR/rerun-newer.log"
    wait_until "version $VERSION_NEWER runs" 120 installed_version_is "$VERSION_NEWER"
    expect_equal "the earlier program is kept as wyo.previous" "$VERSION_FIRST" "$(version_of /usr/local/bin/wyo.previous)"
    expect "a copy of the database was made before the update" test -s "/var/lib/wyo/backups/wyo-before-update-$VERSION_NEWER.db"
    expect "the updater has finished" updater_finished
    expect_equal "the state names the version" "$VERSION_NEWER" "$(state_field latestVersion)"
    expect_equal "no version failed" "" "$(state_field failedVersion)"
    expect "the request file is gone" test ! -e /var/lib/wyo/update-request
    expect_equal "no work folder of the updater is left" "" "$(leftover_work_folders)"
    expect "no staged program is left" test ! -e /usr/local/bin/wyo.new
    expect_equal "the key file is unchanged" "$key_before" "$(sha256sum /var/lib/wyo/secret.key)"
    expect_equal "the owner still signs in with the same password" 200 "$(sign_in "$OWNER_PASSWORD")"
}

# A server set up by an earlier installer has the earlier timer. A run of the installer puts the timer right, and so
# does a run of the updater of the version that runs (a server that only updates itself never runs the installer
# again); neither restarts the wyo server, because only the timer changed.
earlier_timer_is_put_right() {
    step "An earlier timer is put right by the installer and by the updater without restarting the wyo server"
    local pid
    pid=$(main_pid)

    write_earlier_timer
    bash "$DIR/install-test.sh" </dev/null >"$DIR/rerun-timer.log" 2>&1 || { cat "$DIR/rerun-timer.log" >&2; fail "the run of the installer failed"; }
    expect "the installer wrote the program's timer" timer_is_the_programs
    expect "systemd follows the installer's timer" timer_is_moscow
    expect_equal "the installer did not restart the service" "$pid" "$(main_pid)"

    write_earlier_timer
    systemctl start wyo-update.service
    expect "the updater wrote the program's timer" timer_is_the_programs
    expect "systemd follows the updater's timer" timer_is_moscow
    expect_equal "the updater did not restart the service" "$pid" "$(main_pid)"
}

asked_for_programs() { grep -c 'GET /wyo_linux_' "$SERVER_LOG" || true; }

broken_release_is_put_back() {
    step "A release that does not come up is put back"
    serve_release broken
    expect "the owner's press is answered" request_update
    wait_until "the failed version is recorded" 300 state_is failedVersion "$VERSION_BROKEN"
    expect_equal "the earlier program is back" "$VERSION_NEWER" "$(version_of /usr/local/bin/wyo)"
    wait_until "the earlier version answers" 60 healthy
    expect_equal "the owner still signs in after the database was put back" 200 "$(sign_in "$OWNER_PASSWORD")"
    expect "the failure time is recorded" test -n "$(state_field failedAt)"

    local downloads
    downloads=$(asked_for_programs)
    systemctl start wyo-update.service
    expect_equal "the daily run does not try the failed version again" "$downloads" "$(asked_for_programs)"
}

changed_release_is_refused() {
    step "A release with a changed file is refused"
    serve_release changed
    local pid
    pid=$(main_pid)
    if systemctl start wyo-update.service; then fail "the updater accepted a release whose program was changed"; fi
    passed "the updater ended with an error"
    expect_equal "the program is still the earlier one" "$VERSION_NEWER" "$(version_of /usr/local/bin/wyo)"
    expect_equal "the service was not touched" "$pid" "$(main_pid)"
    expect "the state says the file is not the signed one" bash -c "grep -q 'signed' /var/lib/wyo/update-state.json"
}

uninstall_leaves_the_data() {
    step "--uninstall leaves the data"
    local key_before
    key_before=$(sha256sum /var/lib/wyo/secret.key)
    bash "$DIR/install-test.sh" --uninstall </dev/null
    expect "the program is gone" test ! -e /usr/local/bin/wyo
    expect "the units are gone" bash -c 'test ! -e /etc/systemd/system/wyo.service && test ! -e /etc/systemd/system/wyo-update.timer'
    expect "the service is stopped" bash -c '! systemctl is-active --quiet wyo.service'
    expect "the database is still there" test -s /var/lib/wyo/wyo.db
    expect_equal "the key file is unchanged" "$key_before" "$(sha256sum /var/lib/wyo/secret.key)"
}

run_everything() {
    [ "$(id -u)" = 0 ] || fail "the run needs root: sudo -E scripts/ci/installer-job.sh run"
    OWNER_PASSWORD=$(openssl rand -hex 16)
    trap on_exit EXIT
    prepare_machine
    start_pebble
    prepare_releases
    write_test_environment

    step "The installer runs without questions"
    printf '%s\n' "$OWNER_PASSWORD" >"$DIR/owner-password"
    chmod 0600 "$DIR/owner-password"
    bash "$DIR/install-test.sh" --domain "$DOMAIN" --login "$OWNER_LOGIN" --password-file "$DIR/owner-password" --updates auto
    check_installation
    measure
    second_run_changes_nothing
    rerun_installs_newer_through_updater
    earlier_timer_is_put_right
    broken_release_is_put_back
    changed_release_is_refused
    uninstall_leaves_the_data
    step "Everything the job checks passed"
}

case ${1:-} in
    build) build_programs ;;
    run) run_everything ;;
    *) echo "usage: $0 build | run" >&2 && exit 2 ;;
esac
