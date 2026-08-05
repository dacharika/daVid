---
id: rt-claudew
kind: runtime
created: 2026-08-05T14:22:42Z
created_by: a-root
name: claudew
binary: claude
invoke_mode: arg
invoke_flag: -p
invoke_args: "[--allowedTools, \"Read,Grep,Glob,LS,Edit,Write,Bash\", --permission-mode, acceptEdits]"
sandbox_ro_args: "[--allowedTools, \"Read,Grep,Glob,LS,Bash(dacli:*)\"]"
env_passthrough: "[HOME, PATH, USER, LOGNAME, TMPDIR]"
model_flag: --model
skills_native_dir: .claude/skills
usage_format: stream-json
---
# claudew
Flags here are assumptions until `dacli runtime doctor` verifies them against the installed binary.
