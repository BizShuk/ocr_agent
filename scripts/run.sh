#!/usr/bin/env bash
# run:setup — 冪等 (idempotent) 的前置作業，不啟動任何服務。
# 設定的唯一位置是 ~/.config/ocr_agent；repo 內的 tmp/config 只是指過去的 symlink。
# 既有的 settings.json 只補缺少的 key，不覆寫已調整過的 domain 設定。

set -euo pipefail

project_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
config_root="${XDG_CONFIG_HOME:-${HOME}/.config}/ocr_agent"

mkdir -p "$config_root/data" "$config_root/logs" "$project_root/tmp"
ln -sfn "$config_root" "$project_root/tmp/config"

cd "$project_root"
go run . config default --merge

printf 'ocr_agent config link: %s -> %s\n' "$project_root/tmp/config" "$config_root"
