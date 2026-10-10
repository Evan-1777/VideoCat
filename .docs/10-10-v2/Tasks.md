# Tasks：新建公共仓库并启用 CI 自动验证构建

**状态**：DONE（2026-10-10）
**回归测试结论**：本机 `go vet ./...` + `go test ./...` + `npm run build` 通过；CI run 38063227343 success，产物 videocat-windows 上传成功

**关联 Plan**：无（Quick 模式，跳过 Plan）
**总计 Task**：3 个

---

## Phase 1：CI 触发与远端推送

### TASK-001：为工作流添加 push 触发

- **Status**：DONE
- **Executor**：主线
- **Description**：在 `.github/workflows/build-windows.yml` 的 `on:` 中添加 `push.branches: [master]`，使每次推送到 master 均自动运行单测与构建。
- **Details**：
  - 保留既有 tag `v*` 与 `workflow_dispatch` 触发
- **Acceptance Criteria**：
  - YAML 语法有效
  - 推送 master 后 CI 自动启动

### TASK-002：创建公共仓库 VideoCat 并推送

- **Status**：DONE
- **Executor**：主线
- **Description**：用 `gh repo create Evan-1777/VideoCat --public --source . --push` 创建公共仓库并推送当前 master。
- **Details**：
  - 远端协议 https
  - 推送前完成 TASK-001 修改并提交
- **Acceptance Criteria**：
  - 远端存在且为 public
  - master 已推送，`git remote -v` 含 origin

---

## Phase 2：CI 闭环验证与文档

### TASK-003：验证 CI 自动验证与构建通过

- **Status**：DONE
- **Executor**：主线
- **Description**：观察推送触发的 GitHub Actions 运行，确认 go test 与 wails build 全部通过、产物上传成功。
- **Details**：
  - 用 `gh run list` / `gh run watch` 跟踪
  - 失败则修复后重推
  - 首次运行（38062962931）失败，三项根因均已修复：
    1. `main.go` embed `frontend/dist` 在 `go test` 前不存在 → 工作流新增「Build frontend」步骤（`npm ci && npm run build`）
    2. `TestSettingsRoundTrip` / `TestLoadSettingsMissingFile` 依赖 `XDG_CACHE_HOME`，Windows 下 `os.UserCacheDir` 读 `%LocalAppData%`，读写污染真实缓存目录 → 测试补 `t.Setenv("LOCALAPPDATA", cache)`
    3. `TestOutputPaths` 断言硬编码 `/`，Windows 下 `filepath.Join` 产出 `\` → 期望值改用 `filepath.Join` 构造
  - 修复提交 3735f69，本机 `go vet ./...` + `go test ./...` + `npm run build` 全部通过
- **Acceptance Criteria**：
  - CI 运行状态 success
  - 产物 videocat-windows 存在
  - Project.md 中「仓库尚无 remote」相关条目更新
