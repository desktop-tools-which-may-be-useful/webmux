# Shell 目录报告

WebMux 接受 [OSC 7](https://wezterm.org/escape-sequences.html) 的 `file://主机/绝对路径` 报告，不会自动改写 shell 配置。需要时自行把下面片段加入 bash 或 zsh；不启用时终端和会话列表照常工作。

```bash
# bash
_webmux_osc7() {
  local p=${PWD//'%'/'%25'}
  p=${p// /%20}
  p=${p//'#'/'%23'}
  p=${p//'?'/'%3F'}
  printf '\033]7;file://%s%s\007' "${HOST:-${HOSTNAME:-localhost}}" "$p"
}
PROMPT_COMMAND="_webmux_osc7${PROMPT_COMMAND:+;$PROMPT_COMMAND}"
```

```zsh
# zsh
autoload -Uz add-zsh-hook
_webmux_osc7() {
  local p=${PWD//'%'/'%25'}
  p=${p// /%20}
  p=${p//'#'/'%23'}
  p=${p//'?'/'%3F'}
  printf '\033]7;file://%s%s\007' "${HOST:-${HOSTNAME:-localhost}}" "$p"
}
add-zsh-hook precmd _webmux_osc7
```

只报告目录，不采集命令正文。SSH 等子会话可能报告另一台机器的路径，WebMux 按主机名区分本机与远端；远端目录仅用于展示和复制，不参与本机文件定位。进程退出后报告即清除。
