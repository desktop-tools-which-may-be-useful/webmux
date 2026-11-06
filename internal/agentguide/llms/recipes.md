# 常用流程

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
