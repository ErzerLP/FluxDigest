# 首次联调与使用

本文档适用于已经完成 `bash install.sh` 部署后的首次接入与联调。

## 先看安装摘要

部署完成后，先打开安装器输出的 `install-summary.txt`。你需要从这里确认：

- FluxDigest WebUI 地址
- Miniflux 地址
- Halo 地址（若启用）
- 默认管理员用户名与密码
- `.env` 与 `docker-compose.yml` 路径

只有确认这些入口无误，再进行后续联调。

## 登录 FluxDigest WebUI

1. 在浏览器访问安装摘要中的 FluxDigest 地址
2. 使用安装器生成的管理员账号密码登录
3. 进入配置页面，开始录入 LLM、Miniflux 与 Halo 信息

## 在 Miniflux 添加 RSS

1. 打开 FluxDigest WebUI 中的 Miniflux 入口，或直接访问安装摘要中的 Miniflux 地址
2. 在 Miniflux 后台新增 RSS 订阅源
3. 确认 Miniflux 中已经能看到最新文章

> FluxDigest 不负责订阅管理，所有 RSS 的新增、修改、删除都必须在 Miniflux 后台完成。

## 配置 Miniflux 接入

在 FluxDigest WebUI 的 `Configs -> Miniflux` 页面填写：

- `Base URL`
- `Fetch Limit`
- `Lookback Hours`
- `API Token`

保存后执行“测试连接”，应看到连接成功状态。

## 配置 LLM

在 `Configs -> LLM Config` 页面填写：

- `Base URL`
- `Model`
- `Timeout (ms)`
- `API Key`

保存后先执行一次“测试连接”。如果你的 LLM 服务必须走代理，请先在部署环境中配置 `http_proxy` / `https_proxy`。

## 配置 Halo 发布

如果部署组合包含 Halo，请在 `Configs -> Publish` 页面填写：

- `Provider = halo`
- `Halo Base URL`
- `Halo Token`

保存后执行“测试连接”，确认发布通道可用。

## 触发日报验证

安装完成后可以通过 FluxDigest 的作业接口触发一次日报：

```bash
curl -X POST http://<fluxdigest-host>:18088/api/v1/jobs/daily-digest \
  -H "X-API-Key: <APP_JOB_API_KEY>"
```

随后检查：

- `GET /api/v1/articles`
- `GET /api/v1/dossiers`
- `GET /api/v1/digests/latest`

确认文章、单篇分析与日报均已生成。

## 常见问题

### Miniflux 连接失败
- 检查 `Base URL` 和 `API Token`
- 确认 Miniflux 后台已正常抓取文章

### LLM 测试失败
- 检查 Base URL / API Key / Model
- 若需要代理，确认部署环境代理变量已生效

### Halo 发布失败
- 检查 Halo Token 权限
- 检查 Halo Base URL 是否填写正确

### 日报触发 401
- 检查 `APP_JOB_API_KEY` 是否与请求头一致
- 可从安装摘要或 `.env` 中确认实际值
