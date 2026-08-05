#!/bin/zsh
# Local (no-remote) build driver: one loop cycle -> accept everything that
# merged to main (which also frees fixer WIP) -> repeat, until the backlog is
# empty, a STOP file appears, or the iteration cap is hit.
cd ~/Documents/GitHub/cutvec || exit 1
export PATH="$HOME/go/bin:$HOME/.local/bin:/opt/homebrew/bin:$PATH"
LOG=.dacli/drive.log
: > "$LOG"
MAX_ITERS=${1:-8}
for i in $(seq 1 "$MAX_ITERS"); do
  [ -f .dacli/STOP ] && { echo "[$i] STOP file present — halting"; break; }
  open=$(ls .dacli/projects/cutvec/tasks/open/ 2>/dev/null | wc -l | tr -d ' ')
  echo "===== iter $i · open=$open · done=$(ls .dacli/projects/cutvec/tasks/done/ 2>/dev/null | wc -l | tr -d ' ') ====="
  [ "$open" = "0" ] && { echo "backlog empty — done"; break; }
  dacli loop --width 3 --max-cycles 1 >> "$LOG" 2>&1
  # accept every task whose branch merged to main (verify build+test); frees WIP
  for num in $(git branch --merged main 2>/dev/null | grep -oE 'dacli/[0-9]+' | grep -oE '[0-9]+' | sort -u); do
    dacli accept "$num" --verify "go build ./... && go test ./..." >> "$LOG" 2>&1 \
      && echo "  accepted $num"
  done
  dacli sync >> "$LOG" 2>&1
  dacli status 2>&1 | head -1
done
echo "===== FINAL ====="
dacli status 2>&1 | head -1
echo "done: $(ls .dacli/projects/cutvec/tasks/done/ 2>/dev/null | wc -l | tr -d ' ')  open: $(ls .dacli/projects/cutvec/tasks/open/ 2>/dev/null | wc -l | tr -d ' ')"
go build ./... 2>&1 && echo BUILD_OK || echo BUILD_FAIL
go test ./... 2>&1 | tail -8
echo "=== command surface ==="; go run ./cmd/cutvec describe 2>&1 | grep -E '"name"' | head -30
