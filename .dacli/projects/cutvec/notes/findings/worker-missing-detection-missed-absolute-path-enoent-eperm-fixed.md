---
id: f-worker-missing-detection-missed-absolute-path-enoent-eperm-fixed
kind: note
note_kind: finding
created: 2026-08-05T23:11:13Z
created_by: a-fixer-978dgv
about: "[[009]]"
severity: moderate
---
# worker.missing detection missed absolute-path ENOENT/EPERM (fixed)
internal/worker/worker.go:94 keyed worker.missing solely on errors.As(err,*exec.Error), which Go only wraps for PATH lookups (name without a slash). A worker given by an absolute path that does not exist returns a *fs.PathError, so cmd.Run's error fell through to worker.crashed (retryable) — an agent would retry a permanently-missing binary forever. Fixed by also matching errors.Is(err,os.ErrNotExist)||errors.Is(err,os.ErrPermission) (worker.go:99). Regression-covered by TestMissingWorker in internal/worker/worker_test.go which uses an absolute nonexistent path and asserts code==worker.missing, not-retryable.
