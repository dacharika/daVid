#!/bin/zsh
# Local (no-remote) build driver.
# Each iteration: run ONE loop cycle -> accept the tasks that cycle merged into
# main -> retire finished fixer agents (frees WIP; they don't auto-retire without
# a PR merge) -> repeat, until backlog empty / STOP file / iteration cap.
cd ~/Documents/GitHub/cutvec || exit 1
export PATH="$HOME/go/bin:$HOME/.local/bin:/opt/homebrew/bin:$PATH"
LOG=.dacli/drive.log; ITER=.dacli/iter.log
MAX_ITERS=${1:-10}
for i in $(seq 1 "$MAX_ITERS"); do
  [ -f .dacli/STOP ] && { echo "[$i] STOP present — halting"; break; }
  open=$(ls .dacli/projects/cutvec/tasks/open/ 2>/dev/null | wc -l | tr -d ' ')
  done=$(ls .dacli/projects/cutvec/tasks/done/ 2>/dev/null | wc -l | tr -d ' ')
  echo "===== iter $i · open=$open done=$done ====="
  [ "$open" = "0" ] && { echo "backlog empty — done"; break; }

  # free any lingering WIP before the cycle
  for a in $(ls .dacli/agents/ 2>/dev/null | grep -oE 'a-fixer-[a-z0-9]+' | sort -u); do
    dacli agent retire "$a" >/dev/null 2>&1
  done

  dacli loop --width 3 --max-cycles 1 > "$ITER" 2>&1
  cat "$ITER" >> "$LOG"

  # accept exactly the tasks this cycle merged into main
  merged=$(grep -oE 'reclaimed worktree cutvec-[0-9]+' "$ITER" | grep -oE '[0-9]+' | sort -u)
  for num in $merged; do
    dacli accept "$num" --verify "go build ./... && go test ./..." >> "$LOG" 2>&1 \
      && echo "  accepted $num"
  done
  dacli sync >> "$LOG" 2>&1
  dacli status 2>&1 | head -1
done
echo "===== FINAL ====="
dacli status 2>&1 | head -1
echo "done: $(ls .dacli/projects/cutvec/tasks/done/ 2>/dev/null|wc -l|tr -d ' ')  open: $(ls .dacli/projects/cutvec/tasks/open/ 2>/dev/null|wc -l|tr -d ' ')"
go build ./... 2>&1 && echo BUILD_OK || echo BUILD_FAIL
go test ./... 2>&1 | tail -10
echo "=== command surface ==="; go run ./cmd/cutvec describe 2>&1 | grep -E '"name"'
