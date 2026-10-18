# Web 工具平台（独立部署）——前端可行性分析与设计思考

> 本文档整理自 rendering-cms-platform CMS 后台"Web 工具箱"功能的调研结论。经决策，该功能不并入 CMS 后台，而是作为独立平台单独部署。本文档为可迁移的思考沉淀，可整体复制到新项目作为初始需求与技术分析文档，内容自包含、不依赖原项目上下文。

## 1. 平台定位

- 纯前端的在线工具集合，第一版包含四大类工具：
  1. JSON 处理：格式化、压缩、校验；
  2. 图片处理：压缩、格式转换；
  3. 安全工具：密码生成、SSH 密钥对生成、常见加密、常见 Hash 算法；
  4. 日期时间：时间获取、日期格式化、日期转换、时区转换等。
- 核心卖点：**所有数据处理均在用户本地浏览器内完成**，任何输入（密码、密钥、图片、文本）都不经过网络上传。这既是隐私优势，也免除了服务端算力成本与敏感数据合规负担。
- 形态：纯静态 SPA。无后端、无数据库、无登录、无用户体系。

## 2. 核心结论

四类功能**全部可以纯前端实现，零后端 API、零数据库**：

| 功能类别 | 实现路径 | 新增依赖 |
| --- | --- | --- |
| JSON 格式化/压缩 | `JSON.parse` / `JSON.stringify` | 无 |
| 图片压缩/格式转换 | Canvas / `OffscreenCanvas` / `toBlob` | 无（可选 `pica`） |
| 密码生成、加密、Hash | WebCrypto（`crypto.subtle`） | `hash-wasm`（仅 MD5/CRC32/SHA-3 需要） |
| SSH 密钥对 | WebCrypto + OpenSSH 格式拼装，或 `ed25519-keygen` | `ed25519-keygen` |
| 日期时间 | 原生 `Intl` API | 无 |

## 3. 分项技术方案

### 3.1 JSON 格式化 / 压缩（零依赖，最简单）

- 格式化：`JSON.stringify(value, null, 2)`（缩进 2/4 空格可选）；压缩：`JSON.stringify(value)`。
- 附加能力：语法校验并定位错误位置、key 排序、Unicode 转义/反转义、复制与下载结果。
- 注意点：
  - 大输入（数 MB 以上）在主线程解析会卡 UI，用 Web Worker 兜底，或第一版限制输入大小并提示；
  - 容忍注释/尾逗号（JSONC/JSON5）需要额外解析库，第一版不做，严格 JSON 即可。

### 3.2 图片压缩 / 格式转换（Canvas API，零依赖可行）

- 技术路径：`createImageBitmap(blob, { imageOrientation: 'from-image' })` → `OffscreenCanvas` → `canvas.toBlob(type, quality)`。
- 能力范围：
  - 格式互转：PNG ↔ JPEG ↔ WebP（现代浏览器 `toBlob` 均支持这三种编码）；
  - 压缩：JPEG/WebP 可调质量参数；PNG 为无损格式，只能通过缩放尺寸减体积；
  - 附加能力：批量处理、分辨率缩放、压缩前后体积对比、结果下载。EXIF 方向由 `imageOrientation: 'from-image'` 自动摆正。
- 明确限制（需在 UI 中告知）：
  - 浏览器不支持 AVIF 编码（解码可以）；HEIC 解码仅 Safari；
  - 动图（GIF/动画 WebP/APNG）经 Canvas 会丢失动画，不适合此方案；
  - 追求高质量缩放可引入 `pica`（Lanczos，约 100KB，动态 import 按需加载），第一版可不加。

### 3.3 密码生成 / 加密 / Hash / SSH 密钥（WebCrypto 为主）

| 功能 | 方案 | 依赖 |
| --- | --- | --- |
| 密码生成 | `crypto.getRandomValues`，用拒绝采样避免模偏差；长度、字符集、排除易混淆字符等选项 | 无 |
| Hash | SHA-1/SHA-256/SHA-384/SHA-512、HMAC 用 `crypto.subtle.digest`；MD5、CRC32、SHA-3 不在 WebCrypto 中，用 `hash-wasm`（wasm、体积小、支持流式，适合大文件） | hash-wasm（按需加载） |
| 对称加密 | AES-GCM（推荐默认，12 字节 IV）、AES-CBC；口令派生密钥用 PBKDF2 | 无 |
| 非对称加密 | RSA-OAEP 加解密 | 无 |
| SSH 密钥对 | 见下 | ed25519-keygen |

SSH 密钥对生成是本类中复杂度最高的功能：

- WebCrypto 可生成 Ed25519（Chrome 137+/Safari 17+）、RSA、ECDSA 密钥材料，但输出 `-----BEGIN OPENSSH PRIVATE KEY-----` 私钥格式和 `ssh-ed25519 AAAA...` 公钥行，需要手动拼装 OpenSSH 二进制结构（magic header `openssh-key-v1\0`、cipher/kdf 字段、公钥 blob、私钥段），约 100 多行纯 TS。
- 推荐路线：**Ed25519 优先**，用 `ed25519-keygen`（基于 @noble/curves，纯 JS、浏览器友好）直接输出 OpenSSH 格式，不受各浏览器 WebCrypto Ed25519 支持度差异影响；RSA 3072/4096、ECDSA P-256 用 WebCrypto 生成 + 手动拼装作为后续补充。
- 带 passphrase 的私钥需要 bcrypt KDF + AES-256-CTR 加密私钥段，第一版不做（生成明文私钥并在 UI 显著警告），后续版本再支持。
- 环境约束：`crypto.subtle` 仅在安全上下文（HTTPS 或 localhost）可用；独立平台部署在任意 HTTPS 静态托管上即满足。

### 3.4 日期时间（零依赖，原生 Intl 全覆盖）

不需要 dayjs/date-fns：

- 时区转换/展示：`Intl.DateTimeFormat` 配 IANA 时区名（如 `Asia/Shanghai`）即可任意时区互转，浏览器原生完整支持；
- 时区列表：`Intl.supportedValuesOf('timeZone')` 直接获取全部可用时区；
- 其余能力：Unix 时间戳 ↔ 日期互转、自定义格式化输出、相对时间（`Intl.RelativeTimeFormat`，如"3 小时前"）、时间差计算，均原生可做；
- 短板：`Intl` 不负责解析（按自定义格式字符串转 Date 需手写小工具函数），工作量不大。

## 4. 架构与工程约定

- 形态：单页应用（React + TypeScript + Vite 或同等栈均可），静态托管部署，无需任何服务端。
- 路由按工具拆分页 面，全部 lazy 加载；重型依赖（hash-wasm、ed25519-keygen、pica）通过动态 import 按需加载，不进主包。
- 每个工具的核心逻辑写成纯 TS 模块（不依赖 DOM/React），便于单元测试；UI 层只做参数收集与结果展示。
- 建议目录结构：

  ```text
  web-tools/
    src/
      pages/          # 各工具页面，按 json/ image/ crypto/ datetime/ 分子目录
      lib/            # 纯 TS 核心逻辑（json、image、crypto、datetime 各自独立模块）
      components/     # 共享 UI（结果面板、复制按钮、隐私提示等）
      routes/
  ```

- 每个 工具页显著标注"所有处理均在本地浏览器完成，数据不会上传"，密码、密钥、图片类工具尤其必要。
- 无登录、无用户数据，天然规避了敏感数据存储与 IP 隐私问题。

## 5. 部署要求

- 纯静态产物，可部署到任意静态托管（Cloudflare Pages、Netlify、对象存储静态网站等）。
- 必须 HTTPS：`crypto.subtle` 只在安全上下文可用（本地开发 localhost 除外）。
- 若使用 History 路由，静态托管需配置 SPA fallback（所有路径回退到 index.html），或改用 hash 路由规避。

## 6. 实施分期建议（按复杂度递增）

1. 第一期：JSON 工具 + 日期时间工具（零依赖，最快见效）；
2. 第二期：密码生成 + 加密 + Hash（WebCrypto + hash-wasm）；
3. 第三期：图片压缩 / 格式转换（Canvas）；
4. 第四期：SSH 密钥对（Ed25519 优先，OpenSSH 格式拼装）。

## 7. 候选扩展（第一版之后）

Base64/URL 编解码、UUID 生成、JWT 解码、二维码生成与解析、正则测试、文本 Diff、颜色格式转换、单位换算等，均属于同一"纯前端本地处理"模式，可按需增量添加。

## 8. 关键决策记录

- 原方案为并入 rendering-cms-platform CMS 后台（挂 `/admin/tools` 菜单）；经决策改为独立平台部署，理由：与 CMS 业务无耦合、无共享数据、独立演进与部署更自由，且公开工具站无需登录门槛。
- 全部功能前端化的前提成立：四类功能在浏览器端 API（JSON 内置、Canvas、WebCrypto、Intl）覆盖度完整，缺口（MD5/CRC32、OpenSSH 格式）均可由小型纯 JS/wasm 库补齐。
