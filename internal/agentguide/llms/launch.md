# 启动会话

```sh
webmux run --detach -- make test                          # 后台运行，输出 "<ID>\t<链接>"
webmux run --detach --name review --cwd ~/src/app -- codex
webmux run --detach --launch-config "Claude Code"         # 使用 Web 端保存的启动配置（按名称）
webmux run --detach --env VIRTUAL_ENV=/w/.venv -- python app.py
```

- `--` 之后的参数原样作为 argv，不经 shell 展开；需要 shell 特性时显式写 `bash -lc '...'`。
- 自定义命令继承调用方的环境（终端身份变量与 `WEBMUX_*` 除外），`--env` 覆盖同名变量。
- `--launch-config` 的环境由服务端展开，不经过 CLI；它与 `--` 后的命令、`--env` 互斥。
- 不带 `--detach` 时 `run` 会附着当前终端，需要真实终端。agent 的工具调用中请加 `--detach`。

启动配置名称可以在 Web 端设置页的“启动配置”中查看。
