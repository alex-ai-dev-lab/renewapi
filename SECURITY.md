# 安全说明

## 漏洞报告

安全问题请先私下联系[仓库维护者](https://github.com/alex-ai-dev-lab)，
不要在公开 Issue 中披露未修复漏洞、访问令牌或生产数据。
报告请包含受影响版本、组件、复现步骤、影响范围和经过脱敏的必要证据。
本仓库的报告联系人和响应安排不沿用上游项目的邮箱或时限承诺。

## 凭据与部署

- 不提交本地凭据。`Token/`、`.env`、`*.secret`、`*.local.env`、
  `github-auth.env`、`server-access.env`、`id_rsa*`、`*.pem` 已在 Git 和 Docker
  构建上下文的忽略规则中排除。
- 脚本不得使用 `set -x` 输出凭据；日志和问题报告必须隐藏令牌、密码、API Key
  及 Authorization 内容。
- 设置随机 `SESSION_SECRET`，使用 HTTPS，限制管理入口及数据库网络访问，
  按最小权限配置访问令牌，并保留可恢复的数据备份。

## 本地检查

仓库提供 `scripts/secret-scan.sh` 扫描常见高风险凭据格式；这是辅助扫描，
不能代替人工审查或证明构建产物没有秘密。当前 CI 项目以
[发布工作流](.github/workflows/build-release.yml)为准。
