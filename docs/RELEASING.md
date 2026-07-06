# 发布说明约定

GoReleaser 的 `changelog:` 只是按提交信息自动分组罗列（见 `.goreleaser.yaml`），
足够当"有哪些提交"的索引，但不足以让用户一眼看懂**这个版本对我有什么影响**。
`v0.2.0` 的 tag 说明（聚合本地 YAML 订阅来源、运行时目录改名等改动，附带迁移
步骤）是好的范例；`v0.2.1` 只有一行「Release v0.2.1」是反面例子——本文档定下
后续版本统一遵守的最低要求。

## 打 tag 前

写带内容的 annotated tag（而不是 `git tag vX.Y.Z` 轻量 tag），至少包含：

1. **面向用户的变化**：新增/调整的功能，按用户能感知的粒度描述（不是提交粒度）。
2. **Breaking changes**（如有）：单独一段，说明谁会受影响、需要做什么迁移操作，
   参考迁移脚本时给出具体路径（如 `scripts/migrate-runtime-dir.sh`）。
3. **已知限制或后续计划**（如有）：不追求完整，但别把用户会踩到的坑瞒着。

```
git tag -a vX.Y.Z -m "$(cat <<'EOF'
简述本版本主题…

- 变化 1
- 变化 2

BREAKING: ……（如有）
EOF
)"
```

## CI 跑完之后

GoReleaser（`.goreleaser.yaml` 的 `release:` 块）会用 annotated tag 的信息生成
初版 GitHub Release body，前面附加分组后的 changelog。检查生成结果，需要时用
`gh release edit vX.Y.Z --notes-file -` 补充/精简，保证发布页面上看到的说明
和 tag 里写的一致、可读。

## 预发布版本（beta）

`prerelease: auto` 已经处理了 GitHub 侧的 prerelease 标记（tag 带 `-beta.N`
等 semver 预发布后缀即可，见 `internal/selfupdate` 对稳定/预览两条更新渠道的
说明）。多个 beta 最终合并进同一个正式版时，正式版说明里应聚合这些 beta 各自
引入的变化，而不是简单地只写"合并 betas"。
