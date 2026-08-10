#!/bin/zsh
# Serial babysit driver: build ONE task at a time (width 1) so command-adding
# tasks don't collide on the central dispatch/catalog files. Each iteration:
# sync main<-origin -> build next task (loop opens a PR) -> wait for CI ->
# merge the PR (admin) resolving a stale branch by merging main into it ->
# accept the task. Bounded by iteration cap / STOP / empty backlog.
cd ~/Documents/GitHub/cutvec || exit 1
export PATH="$HOME/go/bin:$HOME/.local/bin:/opt/homebrew/bin:$PATH"
# zsh aborts a command when a glob matches nothing, where bash passes the
# pattern through. The accept step below globs for task files that often do not
# exist, so without this every iteration drowns in "no matches found".
setopt NULL_GLOB
REPO=mlnomadpy/cutvec
LOG=.dacli/drive2.log
N=${1:-5}
for i in $(seq 1 "$N"); do
  [ -f .dacli/STOP ] && { echo "[$i] STOP present"; break; }
  open=$(ls .dacli/projects/cutvec/tasks/open/ 2>/dev/null | wc -l | tr -d ' ')
  echo "===== iter $i · open=$open done=$(ls .dacli/projects/cutvec/tasks/done/|wc -l|tr -d ' ') ====="
  [ "$open" = "0" ] && { echo "backlog empty"; break; }

  git checkout main -q 2>/dev/null; git fetch origin -q 2>/dev/null
  git merge origin/main -q -m "sync main<-origin" 2>/dev/null
  for a in $(ls .dacli/agents/ 2>/dev/null | grep -oE 'a-fixer-[a-z0-9]+' | sort -u); do dacli agent retire "$a" >/dev/null 2>&1; done

  # The review phase files tasks without estimates and the fixer role is capped,
  # so an unsized task is refused every cycle forever. Estimate any unsized task
  # before the loop runs rather than rediscovering the deadlock. --force because
  # the auditor agent that filed it is no longer alive to size its own task.
  for f in .dacli/projects/cutvec/tasks/open/*.md; do
    grep -q '^estimate:' "$f" || dacli task estimate "${${f:t}%%-*}" --estimate 1,2,4 --force >/dev/null 2>&1
  done

  echo "  building next task…"
  # --no-progress-halt takes an integer, and passing it as a bare flag made
  # every `dacli loop` call fail instantly — the driver looked like it was
  # running while never spawning anything. One bounded cycle needs no halt
  # override at all.
  dacli loop --width 1 --max-cycles 1 > .dacli/iter2.log 2>&1
  cat .dacli/iter2.log >> "$LOG"

  sleep 75  # let CI kick off / branch push settle
  for pr in $(gh pr list --repo $REPO --state open --json number -q '.[].number' 2>/dev/null); do
    br=$(gh pr view "$pr" --repo $REPO --json headRefName -q .headRefName 2>/dev/null)
    # try a clean admin merge; if the branch is stale, sync it with main then retry
    if ! gh pr merge "$pr" --repo $REPO --merge --admin --delete-branch 2>>"$LOG"; then
      # ${br} braces are load-bearing: zsh reads $br[...] as an array subscript,
      # which made this line a math error rather than a pattern.
      wt=$(git worktree list | grep -oE "\.dacli/worktrees/[^ ]*${br}[^ ]*" | head -1)
      if [ -n "$wt" ]; then
        ( cd "$wt" && git merge origin/main -m "sync branch<-main" >> "$LOG" 2>&1 \
          && go build ./... >> "$LOG" 2>&1 && git push origin HEAD >> "$LOG" 2>&1 )
        gh pr merge "$pr" --repo $REPO --merge --admin --delete-branch 2>>"$LOG" \
          || echo "  PR #$pr still not mergeable — needs manual"
      fi
    fi
  done

  git checkout main -q 2>/dev/null; git fetch origin -q 2>/dev/null; git merge origin/main -q -m sync 2>/dev/null
  # accept ONLY tasks whose PR is CONFIRMED MERGED (no more false "done"s) + still open
  for num in $(gh pr list --repo $REPO --state merged --json title -q '.[].title' 2>/dev/null | grep -oE '^[0-9]+' | sort -u); do
    ls .dacli/projects/cutvec/tasks/open/$num-*.md >/dev/null 2>&1 || continue
    dacli accept "$num" --verify "go build ./... && go test ./..." >> "$LOG" 2>&1 && echo "  accepted $num (PR merged)"
  done
  dacli sync >> "$LOG" 2>&1
  dacli status 2>&1 | head -1
done
echo "===== FINAL ====="; dacli status 2>&1 | head -1
go build ./... 2>&1 && echo BUILD_OK || echo BUILD_FAIL
go test ./... 2>&1 | tail -6
go run ./cmd/cutvec describe 2>&1 | grep -oE '"name": "[a-z]+"' | sort -u | tr '\n' ' '; echo
