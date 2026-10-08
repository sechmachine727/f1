#!/usr/bin/env bash
# Build the argument list for `docker buildx imagetools create`.
#
 # Usage: docker_manifest_args.sh <tags> <image> <digest-dir>
#
 # `<tags>` is the multi-line output of docker/metadata-action's `tags` output.
 # Prints one argument per line: a "-t <tag>" pair for every tag, then
 # "<image>@sha256:<digest>" for every digest file in the directory. imagetools
 # takes tags as `-t` options and treats positional arguments as source images,
 # so tags must carry the flag. Only coreutils and awk are needed.
set -euo pipefail

usage="usage: docker_manifest_args.sh <tags> <image> <digest-dir>"
# Tags may legitimately be empty when metadata-action produced none; the
# empty case is reported below as a hard error instead of a usage error.
tags=${1-}
image=${2:?$usage}
digest_dir=${3:?$usage}

tag_count=$(printf '%s\n' "$tags" | awk 'NF { count++ } END { print count + 0 }')
if [ "$tag_count" -eq 0 ]; then
    echo "docker_manifest_args: no tags given" >&2
    exit 1
fi

shopt -s nullglob
digest_files=("$digest_dir"/*)
if [ ${#digest_files[@]} -eq 0 ]; then
    echo "docker_manifest_args: no digest files in $digest_dir" >&2
    exit 1
fi

printf '%s\n' "$tags" | awk 'NF { print "-t"; print }'
for digest_file in "${digest_files[@]}"; do
    printf '%s@sha256:%s\n' "$image" "$(basename "$digest_file")"
done
