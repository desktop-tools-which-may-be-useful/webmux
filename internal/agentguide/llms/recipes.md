# 常用流程

## 把任务交给另一个 agent 并取回结果

```sh
webmux chat new --agent "Claude Code" --label run=42 --wait --timeout 30m "修复 lint 并汇报改动"
webmux ls --parent self --label run=42
```

## 在持久会话里跑测试并拿到退出码

```sh
webmux run --follow --strip --name tests -- go test ./... > test.log
echo "exit $?"
```

## 驱动一个 shell 会话并读结果

```sh
id=$(webmux run --detach --name build -- bash | cut -f1)
webmux send "$id" 'make build' --enter --wait idle --stable 3s --timeout 20m
webmux logs "$id" --strip --tail 30
```

## 看子 agent 是否在等你

```sh
webmux wait "$id" --until attention --changed --timeout 30m
webmux screen "$id"            # 看授权选择框里有哪些选项
webmux send "$id" --key enter  # 选中默认选项
```

## 在后台跑一个任务，事后清理

```sh
id=$(webmux run --detach --name tests -- make test | cut -f1)
webmux label "$id" run=nightly
# ……之后
webmux ls --label run=nightly --status exited --json
webmux get "$id" --field session.exitCode
```

## 了解自己所在的环境

```sh
webmux get                       # 仓库、分支、启动配置、链接
webmux get --field worktree.path
```

## 找到并汇报交付物

```sh
dir=$(webmux dir self)
cp report.html "$dir/"
echo "报告在临时文件夹的 report.html"
```

## 查看命令的完整用法

```sh
webmux help                      # 全部命令
webmux run --help                # 一个命令的选项、退出码与示例
webmux help --json | jq -r '.commands[].usage'
```
