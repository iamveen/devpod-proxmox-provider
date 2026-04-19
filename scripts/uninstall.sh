#!/usr/bin/env bash
# uninstall.sh — Remove all Proxmox resources created by setup.sh.
# Discovers devpod resources automatically — no flags required.
# Run as root (or with sudo) on the Proxmox host.
#
# Usage:
#   sudo bash uninstall.sh [--dry-run]
#
# Example (pipe over SSH):
#   ssh user@proxmox-host 'sudo bash -s' < uninstall.sh
set -euo pipefail

DRY_RUN=0

# ── helpers ───────────────────────────────────────────────────────────────────
die()  { echo "ERROR: $*" >&2; exit 1; }
log()  { echo "    $*"; }
step() { echo ""; echo "==> $*"; }

run() {
    if [[ $DRY_RUN -eq 1 ]]; then
        log "[dry-run] $*"
    else
        "$@"
    fi
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --dry-run) DRY_RUN=1; shift ;;
        --help|-h) grep '^#' "$0" | sed 's/^# \?//'; exit 0 ;;
        *) die "Unknown option: $1" ;;
    esac
done

# ── preflight ─────────────────────────────────────────────────────────────────
[[ "$EUID" -ne 0 ]] && die "Must be run as root. Try: sudo bash $0"
command -v pveum &>/dev/null || die "pveum not found — run this script on a Proxmox VE host"
command -v qm    &>/dev/null || die "qm not found — run this script on a Proxmox VE host"

[[ $DRY_RUN -eq 1 ]] && echo "" && log "Mode: dry-run (no changes will be made)"

DEVPOD_USER="devpod@pve"
DEVPOD_TOKEN_NAME="devpod"
DEVPOD_TOKEN_ID="${DEVPOD_USER}!${DEVPOD_TOKEN_NAME}"

# ── phase 1: VMs ──────────────────────────────────────────────────────────────
step "Phase 1: VMs"

DEVPOD_VMIDS=$(qm list 2>/dev/null | awk 'NR>1 && $2 ~ /^devpod-/ {print $1}')

if [[ -z "$DEVPOD_VMIDS" ]]; then
    log "No devpod VMs found"
else
    while IFS= read -r vmid; do
        name=$(qm config "$vmid" 2>/dev/null | awk -F': ' '/^name:/{print $2}')
        status=$(qm status "$vmid" 2>/dev/null | awk '{print $2}')
        if [[ "$status" == "running" ]]; then
            run qm stop "$vmid"
            [[ $DRY_RUN -eq 0 ]] && log "Stopped VM ${vmid}"
        fi
        run qm destroy "$vmid" --purge
        [[ $DRY_RUN -eq 0 ]] && log "Destroyed VM ${vmid} (${name})"
    done <<< "$DEVPOD_VMIDS"
fi

# ── phase 2: cloud-init snippets ──────────────────────────────────────────────
step "Phase 2: Cloud-init snippets"

DEVPOD_SNIPPETS=$(find /var/lib/vz/snippets -name 'devpod-*-vendor.yaml' 2>/dev/null || true)

if [[ -z "$DEVPOD_SNIPPETS" ]]; then
    log "No devpod snippets found"
else
    while IFS= read -r f; do
        run rm -f "$f"
        [[ $DRY_RUN -eq 0 ]] && log "Removed ${f}"
    done <<< "$DEVPOD_SNIPPETS"
fi

# ── phase 3: service account ──────────────────────────────────────────────────
step "Phase 3: Service account"

if ! pveum user list --output-format json 2>/dev/null | grep -q "\"userid\":\"${DEVPOD_USER}\""; then
    log "User ${DEVPOD_USER} not found — skipping"
else
    # ACLs first — must be removed before the token, otherwise Proxmox warns
    # about ACL entries referencing a non-existent token when reading user config
    while IFS= read -r acl_line; do
        path=$(awk '{print $1}' <<< "$acl_line")
        type=$(awk '{print $2}' <<< "$acl_line")
        ugid=$(awk '{print $3}' <<< "$acl_line")
        role=$(awk '{print $4}' <<< "$acl_line")
        if [[ "$type" == "user" ]]; then
            run pvesh set /access/acl --path "$path" --roles "$role" --users  "$ugid" --delete 1
        else
            run pvesh set /access/acl --path "$path" --roles "$role" --tokens "$ugid" --delete 1
        fi
        [[ $DRY_RUN -eq 0 ]] && log "Removed ACL: ${role} on ${path} for ${ugid}"
    done < <(pveum acl list 2>/dev/null | awk 'NR>1 && $3 ~ /^devpod@pve/')

    # Token
    if pveum user token list "${DEVPOD_USER}" --output-format json 2>/dev/null | grep -q "\"tokenid\":\"${DEVPOD_TOKEN_NAME}\""; then
        run pveum user token remove "${DEVPOD_USER}" "${DEVPOD_TOKEN_NAME}"
        [[ $DRY_RUN -eq 0 ]] && log "Removed token ${DEVPOD_TOKEN_ID}"
    else
        log "Token ${DEVPOD_TOKEN_ID} not found — skipping"
    fi

    # User
    run pveum user del "${DEVPOD_USER}"
    [[ $DRY_RUN -eq 0 ]] && log "Removed user ${DEVPOD_USER}"
fi

echo ""
if [[ $DRY_RUN -eq 1 ]]; then
    echo "✓ Dry run complete — no changes were made"
else
    echo "✓ Uninstall complete"
fi
echo ""
