#!/bin/sh
set -eu

# GeneralUser GS 2.0.3, official author source, pinned and checked. Its license
# permits music creation and software-project use; retain the complete license.
lyra_font_dir=${1:?usage: install-soundfont.sh destination-directory}
lyra_font_sha=9575028c7a1f589f5770fccc8cff2734566af40cd26ed836944e9a5152688cfe
lyra_font_url=https://raw.githubusercontent.com/mrbumpy409/GeneralUser-GS/684543d5e5efaef08d02be50dcda8d552478fa60/GeneralUser-GS.sf2
lyra_font_path=$lyra_font_dir/GeneralUser-GS.sf2

if [ -f "$lyra_font_path" ]; then
  lyra_font_actual=$(shasum -a 256 "$lyra_font_path" | cut -d ' ' -f 1)
  if [ "$lyra_font_actual" != "$lyra_font_sha" ]; then
    printf '%s\n' "Existing SoundFont differs from the pinned default: $lyra_font_path" 'Use --soundfont for a custom bank; the existing file was preserved.' >&2
    exit 1
  fi
else
  install -d "$lyra_font_dir"
  lyra_font_tmp=$(mktemp "$lyra_font_dir/.download.XXXXXXXX")
  trap 'rm -f "$lyra_font_tmp"' EXIT HUP INT TERM
  curl --fail --location --retry 2 --connect-timeout 15 --max-time 180 --output "$lyra_font_tmp" "$lyra_font_url"
  lyra_font_actual=$(shasum -a 256 "$lyra_font_tmp" | cut -d ' ' -f 1)
  if [ "$lyra_font_actual" != "$lyra_font_sha" ]; then
    printf '%s\n' 'SoundFont checksum mismatch; nothing was installed.' >&2
    exit 1
  fi
  chmod 0644 "$lyra_font_tmp"
  mv "$lyra_font_tmp" "$lyra_font_path"
fi
install -m 0644 docs/licenses/GeneralUser-GS.txt "$lyra_font_dir/LICENSE.txt"
printf 'SoundFont ready: %s\n' "$lyra_font_path"
