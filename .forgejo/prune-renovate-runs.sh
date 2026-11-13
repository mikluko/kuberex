#!/bin/sh
# Deletes the scheduled runs of the renovate workflow that moved nothing.
#
# A run that pins something pushes a commit by forgejo-actions whose parent is
# the commit the run started on, and no later run starts on that commit: on
# such a commit the newest run is the one that pinned. Every other successful
# scheduled run moved nothing. Failed runs and runs started by hand are kept.
#
# Reads SERVER, REPOSITORY and TOKEN, and the history of the checked-out
# branch. With DRY_RUN set it prints the ids it would delete.
set -eu

api="$SERVER/api/v1/repos/$REPOSITORY/actions/runs"

auth() {
  printf 'header = "Authorization: Bearer %s"\n' "$TOKEN"
}

pinned_from=$(git log --author=forgejo-actions --format=%P | jq -R . | jq -sc .)

# The listing answers with every run unless it is paged, and its event filter
# does not know scheduled runs: their event is "push", their trigger_event
# "schedule".
auth | curl -fsS -K - "$api?workflow_id=renovate.yaml" |
  jq -r --argjson pinned_from "$pinned_from" '
    .workflow_runs
    | ( group_by(.commit_sha)
        | map(max_by(.id))
        | map(select(.commit_sha as $sha | $pinned_from | index($sha)) | .id)
      ) as $pinned
    | .[]
    | select(.trigger_event == "schedule" and .status == "success")
    | select(.id as $id | $pinned | index($id) | not)
    | .id' |
  while read -r id; do
    if [ -n "${DRY_RUN:-}" ]; then
      echo "$id"
    else
      auth | curl -fsS -K - -X DELETE "$api/$id"
    fi
  done
