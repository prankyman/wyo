#!/usr/bin/env bash
# The wyo installer. Two commands on the seller's server (README.md explains each):
#   curl -fsSL -o install.sh https://github.com/prankyman/wyo/releases/latest/download/install.sh
#   sudo bash install.sh
#
# What it does: looks at the server before it asks anything; asks four things (the domain, the owner's login, the
# owner's password, whether updates install themselves); downloads the wyo server for this processor and checks it
# against a signed list; sets up the system user, the folders, the four system units and the owner; starts the wyo
# server and waits until it answers. Run again, it repairs and never destroys. It sends nothing anywhere but its
# downloads from the release address below.
#
# The order of checking a release is written here and once in Go (internal/release): the signature of checksums.txt
# first, then each downloaded file's SHA-256 against it; a file that fails is deleted and nothing is installed.
set -euo pipefail

# ---- The two constants a test copy of this script replaces (scripts/ci/installer-job.sh); nothing else changes them ----
RELEASE_URL='https://github.com/prankyman/wyo/releases/latest/download'
PUBLIC_KEY_B64='MCowBQYDK2VwAyEAKAD2JS/mnf2iy29lFfyBMY02Fq/bq+kc1wuNwNH+aBc='

# ---- Every sentence the installer says, in one table. %s is filled in by the caller. ----
declare -A T=(
    [need_root]='Установщик нужно запускать от имени администратора: sudo bash install.sh'
    [need_systemd]='На этом сервере нет systemd. Установщик поддерживает только системы с systemd (Debian 12 и 13, Ubuntu 22.04 и новее, семейство Red Hat от версии 9).'
    [need_arch]='Процессор %s не поддерживается. Нужен x86_64 (amd64) или aarch64 (arm64).'
    [need_openssl]='Нужен OpenSSL версии 3 или новее: им проверяется подпись выпуска. Сейчас: %s. Обновите систему или поставьте пакет openssl 3.'
    [need_tool]='Не найдена команда «%s». Поставьте её пакетом системы и запустите установщик снова.'
    [port_busy]='Порт %s занят другой программой. wyo сам получает сертификат и слушает порты 80 и 443. Освободите их или запустите установщик с флагом --behind-proxy (тогда wyo слушает 127.0.0.1:8484 без шифрования, а сертификатом занимается ваш веб-сервер; см. docs/behind-a-proxy.md).'
    [server_address]='Адреса этого сервера: %s.'
    [domain_points]='Домен %s указывает на: %s.'
    [domain_here]='Это адрес этого сервера: сертификат будет получен без помех.'
    [domain_elsewhere]='Это не похоже на адрес этого сервера. Если сервер стоит за общим адресом провайдера, так бывает. Иначе поправьте запись A у регистратора домена: сертификат не будет получен, пока домен указывает не сюда.'
    [domain_unknown]='Домен %s пока не указывает ни на какой адрес. Создайте у регистратора запись A с адресом этого сервера. Установка продолжится, wyo будет пробовать получить сертификат, пока запись не появится.'
    [ask_domain]='Доменное имя, под которым wyo будет доступен (например, orders.example.ru): '
    [ask_login]='Логин владельца, 3–64 символа: '
    [ask_password]='Пароль владельца, не короче 10 символов (при вводе не виден): '
    [ask_password_again]='Пароль ещё раз: '
    [ask_updates]='Устанавливать новые версии wyo автоматически? [да/нет, по умолчанию да]: '
    [bad_domain]='«%s» не похоже на доменное имя. Нужно имя из латинских букв, цифр и дефисов с точками, например orders.example.ru.'
    [bad_login]='Логин должен быть длиной от 3 до 64 символов.'
    [bad_password]='Пароль должен быть не короче 10 символов.'
    [passwords_differ]='Пароли не совпали. Попробуйте ещё раз.'
    [bad_updates]='Ответьте «да» или «нет».'
    [no_questions]='Нужно ответить на вопросы, а установщик запущен без терминала. Передайте %s флагами: --domain, --login, --password-file.'
    [password_file]='Не удалось прочитать файл с паролем: %s'
    [unknown_flag]='Неизвестный флаг: %s. Список флагов: bash install.sh --help'
    [flag_needs_value]='Флагу %s нужно значение.'
    [downloading]='Скачиваю выпуск для процессора %s…'
    [download_failed]='Не удалось скачать %s. Проверьте, что сервер выходит в интернет и что github.com доступен.'
    [bad_signature]='ПОДПИСЬ ВЫПУСКА НЕ СОШЛАСЬ. Ничего не установлено. Не запускайте скачанные файлы; сообщите об этом на https://github.com/prankyman/wyo/issues.'
    [file_changed]='Файл %s не совпадает с подписанным списком. Он удалён, ничего не установлено.'
    [file_not_listed]='Подписанный список не содержит файла %s. Ничего не установлено.'
    [bad_version]='В выпуске записана непонятная версия: %s. Ничего не установлено.'
    [verified]='Подпись и контрольные суммы сошлись. Версия: %s.'
    [keep_newer]='Установлена более новая версия (%s), чем в выпуске (%s). Программу не заменяю.'
    [installing]='Устанавливаю…'
    [owner_created]='Владелец создан.'
    [owner_failed]='Не удалось создать владельца. Создайте его командой: sudo -u wyo wyo owner create --login ЛОГИН'
    [waiting_local]='Жду, пока wyo ответит…'
    [local_failed]='wyo не ответил за %s секунд. Посмотрите журнал: sudo journalctl -u wyo -n 50 --no-pager'
    [waiting_public]='Жду, пока адрес %s ответит по https (сертификат выпускается до минуты)…'
    [public_pending]='Адрес %s ещё не отвечает по https. wyo работает и продолжает получать сертификат. Проверьте, что домен указывает на этот сервер и что порты 80 и 443 открыты, затем выполните: sudo wyo doctor'
    [done_title]='Готово. wyo установлен.'
    [done_address]='Адрес:   %s'
    [done_login]='Логин:   %s'
    [done_remember]='Запомните три вещи:'
    [done_forgot]='  1. Забыли пароль: sudo wyo owner reset-password'
    [done_copies]='  2. Копии базы данных лежат в /var/lib/wyo/backups (семь последних, раз в сутки).'
    [done_key]='  3. Скопируйте файл /var/lib/wyo/secret.key в надёжное место: без него сохранённые ключи маркетплейсов нельзя прочитать из копии.'
    [uninstalled]='Программа и системные юниты удалены. Данные в /var/lib/wyo сохранены. Чтобы установить снова, запустите установщик ещё раз.'
    [purge_ask]='Это удалит ВСЕ данные wyo (базу, ключевой файл, сертификат) без возможности восстановления. Чтобы подтвердить, введите доменное имя %s: '
    [purge_refused]='Доменное имя введено неверно. Ничего не удалено.'
    [purge_done]='Все данные wyo удалены.'
    [purge_needs_uninstall]='Флаг --purge работает только вместе с --uninstall.'
    [not_installed]='wyo на этом сервере не установлен.'
    [usage]='Установка: sudo bash install.sh [флаги]
  --domain ИМЯ           доменное имя (иначе спросим)
  --login ЛОГИН          логин владельца (иначе спросим)
  --password-file ФАЙЛ   файл, первая строка которого — пароль владельца (иначе спросим)
  --updates auto|manual  устанавливать новые версии автоматически (по умолчанию auto)
  --behind-proxy         порты 80 и 443 занимает ваш веб-сервер: wyo слушает 127.0.0.1:8484
  --uninstall            удалить программу и юниты, данные оставить
  --purge                вместе с --uninstall: удалить и данные (нужно ввести доменное имя)
  --confirm-domain ИМЯ   подтверждение для --purge без вопросов
  --help                 эта справка'
)

# ---- Fixed names ----
BIN=/usr/local/bin/wyo
ENV_DIR=/etc/wyo
ENV_FILE=$ENV_DIR/wyo.env
OWNER_MARK=$ENV_DIR/.owner-created
DATA=/var/lib/wyo
UNIT_DIR=/etc/systemd/system
SERVICE_USER=wyo
LOCAL_WAIT=90
PUBLIC_WAIT=120

# say KEY [ARGS…] prints a sentence of the table. The format comes from the table, not from user input.
say() {
    local key=$1
    shift
    # shellcheck disable=SC2059 # the table's own sentences are the format string
    printf "${T[$key]}\n" "$@"
}

die() {
    say "$@" >&2
    exit 1
}

TMP=''
cleanup() {
    if [ -n "$TMP" ]; then rm -rf -- "$TMP"; fi
}
trap cleanup EXIT

# The folder for downloads and for what must not outlive the run: a password file, the unit texts.
work_folder() {
    if [ -z "$TMP" ]; then
        TMP=$(mktemp -d)
    fi
}

# ---- Arguments ----
DOMAIN='' LOGIN='' PASSWORD_FILE='' UPDATES='' BEHIND_PROXY=0 UNINSTALL=0 PURGE=0 CONFIRM_DOMAIN=''
ARCH='' VERSION='' ADDRESS=''

parse_arguments() {
    while [ $# -gt 0 ]; do
        case $1 in
            --domain | --login | --password-file | --updates | --confirm-domain)
                [ $# -ge 2 ] || die flag_needs_value "$1"
                case $1 in
                    --domain) DOMAIN=$2 ;;
                    --login) LOGIN=$2 ;;
                    --password-file) PASSWORD_FILE=$2 ;;
                    --updates) UPDATES=$2 ;;
                    --confirm-domain) CONFIRM_DOMAIN=$2 ;;
                esac
                shift 2
                ;;
            --behind-proxy) BEHIND_PROXY=1 && shift ;;
            --uninstall) UNINSTALL=1 && shift ;;
            --purge) PURGE=1 && shift ;;
            --help | -h) say usage && exit 0 ;;
            *) die unknown_flag "$1" ;;
        esac
    done
    if [ "$PURGE" = 1 ] && [ "$UNINSTALL" = 0 ]; then
        die purge_needs_uninstall
    fi
}

# ---- Looking at the server before asking anything ----
look_at_server() {
    [ "$(id -u)" = 0 ] || die need_root
    if [ ! -d /run/systemd/system ] || ! command -v systemctl >/dev/null 2>&1; then
        die need_systemd
    fi

    local machine
    machine=$(uname -m)
    case $machine in
        x86_64 | amd64) ARCH=amd64 ;;
        aarch64 | arm64) ARCH=arm64 ;;
        *) die need_arch "$machine" ;;
    esac

    local tool
    for tool in curl openssl sha256sum install runuser useradd ss ip awk getent; do
        command -v "$tool" >/dev/null 2>&1 || die need_tool "$tool"
    done

    local openssl_version
    openssl_version=$(openssl version 2>/dev/null || true)
    case $openssl_version in
        'OpenSSL '[3-9].* | 'OpenSSL '[1-9][0-9].*) ;;
        *) die need_openssl "${openssl_version:-?}" ;;
    esac
}

# The ports are free, or wyo itself (an earlier run of this script) holds them.
check_ports() {
    if systemctl is-active --quiet wyo.service 2>/dev/null; then
        return 0
    fi
    local port
    local -a ports=(80 443)
    if [ "$BEHIND_PROXY" = 1 ]; then ports=(8484); fi
    for port in "${ports[@]}"; do
        if [ -n "$(ss -H -ltn "sport = :$port")" ]; then
            die port_busy "$port"
        fi
    done
}

own_addresses() {
    ip -o -4 addr show scope global 2>/dev/null | awk '{sub(/\/.*/, "", $4); print $4}' | paste -sd' ' - || true
}

# What the domain points to, one address per word.
domain_addresses() {
    getent ahostsv4 "$1" 2>/dev/null | awk '{print $1}' | sort -u | paste -sd' ' - || true
}

report_domain() {
    local own pointed address
    own=$(own_addresses)
    if [ -n "$own" ]; then say server_address "$own"; fi
    pointed=$(domain_addresses "$1")
    if [ -z "$pointed" ]; then
        say domain_unknown "$1"
        return 0
    fi
    say domain_points "$1" "$pointed"
    for address in $pointed; do
        case " $own " in
            *" $address "*)
                say domain_here
                return 0
                ;;
        esac
    done
    say domain_elsewhere
}

# ---- The answers ----
valid_domain() {
    [[ $1 =~ ^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z][a-z0-9-]*[a-z0-9]$ ]]
}

lower() {
    printf '%s' "$1" | tr '[:upper:]' '[:lower:]'
}

# ask KEY [silent]: prints the sentence of the table as a prompt and returns the answer.
ask() {
    local key=$1 silent=${2:-0} answer
    # shellcheck disable=SC2059 # the table's own sentence is the format string
    printf "${T[$key]}" >&2
    if [ "$silent" = 1 ]; then
        IFS= read -rs answer || answer=''
        printf '\n' >&2
    else
        IFS= read -r answer || answer=''
    fi
    printf '%s' "$answer"
}

read_existing_choices() {
    EXISTING_DOMAIN=''
    EXISTING_PROXY=0
    if [ -f "$ENV_FILE" ]; then
        EXISTING_DOMAIN=$(sed -n 's/^WYO_DOMAIN=//p' "$ENV_FILE" | head -n 1)
        if grep -q '^WYO_BEHIND_PROXY=1' "$ENV_FILE"; then EXISTING_PROXY=1; fi
    fi
    if [ "$EXISTING_PROXY" = 1 ]; then BEHIND_PROXY=1; fi
    if [ -z "$DOMAIN" ]; then DOMAIN=$EXISTING_DOMAIN; fi
}

ask_domain() {
    if [ -n "$DOMAIN" ]; then return 0; fi
    [ -t 0 ] || die no_questions 'домен'
    while :; do
        DOMAIN=$(lower "$(ask ask_domain)")
        if valid_domain "$DOMAIN"; then break; fi
        say bad_domain "$DOMAIN" >&2
    done
}

ask_login() {
    if [ -z "$LOGIN" ]; then
        [ -t 0 ] || die no_questions 'логин и пароль'
        while :; do
            LOGIN=$(ask ask_login)
            if [ ${#LOGIN} -ge 3 ] && [ ${#LOGIN} -le 64 ]; then break; fi
            say bad_login >&2
        done
    fi
    if [ ${#LOGIN} -lt 3 ] || [ ${#LOGIN} -gt 64 ]; then die bad_login; fi
}

ask_password() {
    work_folder
    if [ -n "$PASSWORD_FILE" ]; then
        [ -r "$PASSWORD_FILE" ] || die password_file "$PASSWORD_FILE"
        if [ "$(head -n 1 "$PASSWORD_FILE" | tr -d '\n' | wc -c)" -lt 10 ]; then die bad_password; fi
        return 0
    fi
    [ -t 0 ] || die no_questions 'пароль'
    local first second
    while :; do
        first=$(ask ask_password 1)
        second=$(ask ask_password_again 1)
        if [ "$first" != "$second" ]; then
            say passwords_differ >&2
        elif [ ${#first} -lt 10 ]; then
            say bad_password >&2
        else
            break
        fi
    done
    PASSWORD_FILE=$TMP/owner-password
    (umask 077 && printf '%s\n' "$first" >"$PASSWORD_FILE")
}

ask_updates() {
    if [ -z "$UPDATES" ]; then
        UPDATES=auto
        if [ -t 0 ]; then
            while :; do
                case $(lower "$(ask ask_updates)") in
                    '' | да | д | yes | y) UPDATES=auto && break ;;
                    нет | н | no | n) UPDATES=manual && break ;;
                    *) say bad_updates >&2 ;;
                esac
            done
        fi
    fi
    case $UPDATES in auto | manual) ;; *) die bad_updates ;; esac
}

gather_answers() {
    read_existing_choices
    ask_domain
    DOMAIN=$(lower "$DOMAIN")
    valid_domain "$DOMAIN" || die bad_domain "$DOMAIN"
    report_domain "$DOMAIN"

    if [ -f "$OWNER_MARK" ]; then
        return 0 # the owner exists: the login, the password and the updates choice are not asked again
    fi
    ask_login
    ask_password
    ask_updates
}

# ---- Download and check, in the order of the signed list ----
download_and_check() {
    work_folder
    say downloading "$ARCH"
    local name
    for name in checksums.txt checksums.txt.sig VERSION "wyo_linux_$ARCH"; do
        curl -fsSL --retry 3 --connect-timeout 20 -o "$TMP/$name" "$RELEASE_URL/$name" || die download_failed "$name"
    done

    printf -- '-----BEGIN PUBLIC KEY-----\n%s\n-----END PUBLIC KEY-----\n' "$PUBLIC_KEY_B64" >"$TMP/public.pem"
    if ! openssl pkeyutl -verify -pubin -inkey "$TMP/public.pem" -rawin -in "$TMP/checksums.txt" \
        -sigfile "$TMP/checksums.txt.sig" >/dev/null 2>&1; then
        rm -f -- "$TMP/checksums.txt" "$TMP/checksums.txt.sig" "$TMP/VERSION" "$TMP/wyo_linux_$ARCH"
        die bad_signature
    fi

    for name in VERSION "wyo_linux_$ARCH"; do
        check_file "$name"
    done
    VERSION=$(tr -d '[:space:]' <"$TMP/VERSION")
    [[ $VERSION =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || die bad_version "$VERSION"
    say verified "$VERSION"
}

# check_file NAME: the file's SHA-256 is the one in the signed list; a file that is not is deleted.
check_file() {
    local name=$1 want got
    want=$(awk -v n="$name" '$2 == n || $2 == "*" n {print tolower($1); exit}' "$TMP/checksums.txt")
    if [ -z "$want" ]; then
        rm -f -- "$TMP/$name"
        die file_not_listed "$name"
    fi
    got=$(sha256sum "$TMP/$name" | awk '{print $1}')
    if [ "$got" != "$want" ]; then
        rm -f -- "$TMP/$name"
        die file_changed "$name"
    fi
}

# ---- Setting up ----
CHANGED=0

# put_file SOURCE DEST MODE OWNER:GROUP copies only when the content, the mode or the owner differ, so that a second
# run of the installer changes nothing.
put_file() {
    local source=$1 dest=$2 mode=$3 owner=$4
    if [ -f "$dest" ] && cmp -s "$source" "$dest" && [ "$(stat -c %a "$dest")" = "$mode" ] &&
        [ "$(stat -c %U:%G "$dest")" = "$owner" ]; then
        return 0
    fi
    install -m "$mode" -o "${owner%%:*}" -g "${owner##*:}" "$source" "$dest"
    CHANGED=1
}

version_of_installed() {
    if [ -x "$BIN" ]; then
        "$BIN" version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -n 1 || true
    fi
}

install_program() {
    local installed
    installed=$(version_of_installed)
    if [ -n "$installed" ] && [ "$installed" != "$VERSION" ] &&
        [ "$(printf '%s\n%s\n' "$installed" "$VERSION" | sort -V | tail -n 1)" = "$installed" ]; then
        say keep_newer "$installed" "$VERSION"
        return 0
    fi
    if [ -f "$BIN" ] && ! cmp -s "$TMP/wyo_linux_$ARCH" "$BIN"; then
        cp -p "$BIN" "$BIN.previous"
    fi
    put_file "$TMP/wyo_linux_$ARCH" "$BIN" 755 root:root
}

setup_user_and_folders() {
    if ! id "$SERVICE_USER" >/dev/null 2>&1; then
        useradd --system --user-group --no-create-home --home-dir "$DATA" \
            --shell "$(command -v nologin || echo /usr/sbin/nologin)" "$SERVICE_USER"
    fi
    install -d -m 0750 -o "$SERVICE_USER" -g "$SERVICE_USER" "$DATA"
    install -d -m 0700 -o "$SERVICE_USER" -g "$SERVICE_USER" "$DATA/certs" "$DATA/backups"
    install -d -m 0755 -o root -g root "$ENV_DIR"
}

write_environment() {
    {
        printf 'WYO_DOMAIN=%s\n' "$DOMAIN"
        printf 'WYO_DATA_DIR=%s\n' "$DATA"
        if [ "$BEHIND_PROXY" = 1 ]; then printf 'WYO_BEHIND_PROXY=1\n'; fi
    } >"$TMP/wyo.env"
    put_file "$TMP/wyo.env" "$ENV_FILE" 640 "root:$SERVICE_USER"
}

write_units() {
    cat >"$TMP/wyo.service" <<UNIT
[Unit]
Description=wyo server
After=network-online.target
Wants=network-online.target

[Service]
User=$SERVICE_USER
Group=$SERVICE_USER
EnvironmentFile=$ENV_FILE
ExecStart=$BIN serve
Restart=on-failure
RestartSec=5
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
NoNewPrivileges=true
PrivateTmp=true
ProtectHome=true
ProtectSystem=full

[Install]
WantedBy=multi-user.target
UNIT
    cat >"$TMP/wyo-update.service" <<UNIT
[Unit]
Description=wyo updater

[Service]
Type=oneshot
EnvironmentFile=$ENV_FILE
ExecStart=$BIN update
UNIT
    cat >"$TMP/wyo-update.timer" <<UNIT
[Unit]
Description=wyo daily look for a newer version

[Timer]
OnCalendar=daily
RandomizedDelaySec=24h
Persistent=true

[Install]
WantedBy=timers.target
UNIT
    cat >"$TMP/wyo-update.path" <<UNIT
[Unit]
Description=wyo update requested from the interface

[Path]
PathExists=$DATA/update-request
Unit=wyo-update.service

[Install]
WantedBy=multi-user.target
UNIT
    local unit
    for unit in wyo.service wyo-update.service wyo-update.timer wyo-update.path; do
        put_file "$TMP/$unit" "$UNIT_DIR/$unit" 644 root:root
    done
}

# The wyo server's own user runs these commands from the root folder: the folder the seller stands in may be closed to it.
as_service_user() {
    (cd / && runuser -u "$SERVICE_USER" -- "$@")
}

create_owner() {
    if [ -f "$OWNER_MARK" ]; then return 0; fi
    # The service user reads the password from a copy of its own: the seller's file stays as it is.
    local copy=$TMP/owner-password-copy
    (umask 077 && head -n 1 "$PASSWORD_FILE" >"$copy")
    chown "$SERVICE_USER" "$copy"
    chmod 0711 "$TMP"
    if as_service_user "$BIN" owner create --data "$DATA" --login "$LOGIN" --password-file "$copy"; then
        : >"$OWNER_MARK"
        say owner_created
    else
        die owner_failed
    fi
}

# wait_for URL SECONDS: asks until the address answers with a success status.
wait_for() {
    local url=$1 seconds=$2 spent=0
    while [ "$spent" -lt "$seconds" ]; do
        if curl -fsS --max-time 5 -o /dev/null "$url" 2>/dev/null; then return 0; fi
        sleep 2
        spent=$((spent + 2))
    done
    return 1
}

start_service() {
    systemctl daemon-reload
    systemctl enable wyo.service wyo-update.timer wyo-update.path >/dev/null 2>&1
    if ! systemctl is-active --quiet wyo.service; then
        systemctl start wyo.service
    elif [ "$CHANGED" = 1 ]; then
        systemctl restart wyo.service
    fi
    systemctl start wyo-update.timer wyo-update.path
}

wait_for_server() {
    local local_url=http://127.0.0.1/healthz
    if [ "$BEHIND_PROXY" = 1 ]; then local_url=http://127.0.0.1:8484/healthz; fi
    say waiting_local
    wait_for "$local_url" "$LOCAL_WAIT" || die local_failed "$LOCAL_WAIT"
    ADDRESS=https://$DOMAIN
    if [ "$BEHIND_PROXY" = 1 ]; then
        return 0 # the certificate is the web server's: nothing of ours to wait for
    fi
    say waiting_public "$DOMAIN"
    wait_for "$ADDRESS/healthz" "$PUBLIC_WAIT" || say public_pending "$DOMAIN"
}

install_all() {
    look_at_server
    check_ports
    gather_answers
    download_and_check
    say installing
    setup_user_and_folders
    install_program
    write_environment
    write_units
    as_service_user "$BIN" init --data "$DATA" "--updates=${UPDATES:-auto}"
    create_owner
    start_service
    wait_for_server
    echo
    say done_title
    say done_address "$ADDRESS"
    if [ -n "$LOGIN" ]; then say done_login "$LOGIN"; fi
    say done_remember
    say done_forgot
    say done_copies
    say done_key
}

# ---- Taking it away ----
uninstall_all() {
    [ "$(id -u)" = 0 ] || die need_root
    if [ ! -x "$BIN" ] && [ ! -f "$UNIT_DIR/wyo.service" ]; then
        say not_installed
    fi
    local unit
    for unit in wyo-update.path wyo-update.timer wyo-update.service wyo.service; do
        systemctl disable --now "$unit" >/dev/null 2>&1 || true
        rm -f -- "$UNIT_DIR/$unit"
    done
    systemctl daemon-reload
    rm -f -- "$BIN" "$BIN.previous" "$BIN.new"
    if [ "$PURGE" = 0 ]; then
        say uninstalled
        return 0
    fi
    local domain typed=$CONFIRM_DOMAIN
    domain=$(sed -n 's/^WYO_DOMAIN=//p' "$ENV_FILE" 2>/dev/null | head -n 1 || true)
    if [ -z "$typed" ]; then
        [ -t 0 ] || die purge_refused
        # shellcheck disable=SC2059 # the table's own sentence is the format string
        printf "${T[purge_ask]}" "$domain" >&2
        IFS= read -r typed || typed=''
    fi
    if [ -z "$domain" ] || [ "$typed" != "$domain" ]; then
        die purge_refused
    fi
    rm -rf -- "$DATA" "$ENV_DIR"
    if id "$SERVICE_USER" >/dev/null 2>&1; then userdel "$SERVICE_USER" >/dev/null 2>&1 || true; fi
    say purge_done
}

main() {
    parse_arguments "$@"
    if [ "$UNINSTALL" = 1 ]; then
        uninstall_all
    else
        install_all
    fi
}

main "$@"
