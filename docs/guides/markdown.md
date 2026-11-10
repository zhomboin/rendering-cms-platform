# 文章 Markdown 规则

CMS 编辑预览、后台阅读与 Rendering 博客使用同一份受控 MDX 核心。两仓保存相同的核心源码和契约摘要；修改时须同步两仓，并通过相同回归。前端技术栈与本地启动见[开发环境](../operations/development-environment.md)。

## 支持内容

- 一至六级标题、Setext 标题、加粗、斜体、删除线、引用、分隔线、链接和图片。
- 有序、无序、嵌套和任务列表；反引号或波浪号围栏代码、缩进代码和行内代码。
- GFM 表格、转义管道和单元格对齐。宽表格在独立区域横向滚动，可用键盘聚焦。
- 无属性的 br 换行与 u 下划线；普通换行标签自动规范为自闭合写法。
- JVM 构造及静态初始化名称按文本展示，不影响代码块与行内代码。
- Figure、Gallery 组件只接受静态参数；Gallery 的图片集合只允许静态字面量。

任意脚本、导入导出、执行表达式、属性展开、事件属性和不受支持的 HTML 标签都会报错，不会执行。链接和图片不接受危险协议。

## 标题与目录

目录覆盖一至六级标题，并识别 Setext 标题。链接文字、加粗和行内代码以可读文字生成锚点；中文标题使用稳定标识，重复标题自动追加后缀。代码块里的标题不会进入目录。目录与正文使用同一套锚点生成逻辑。

## 预览与发布

预览解析失败时显示错误；修改后重新解析并清除错误。异步完成的旧结果不能覆盖新正文。后台阅读使用相同规则和目录。

CMS 保存与发布不会替代博客页面生成。博客生成失败可能继续展示已有缓存；不能通过反复清缓存解决正文语法错误。Pagefind 仍需独立重建，见[博客发布对接](../operations/rendering-blog-publishing-integration.md)。

## 实现与验证

核心位于 [解析器](../../frontend/src/markdown/engine.js)、[安全规则](../../frontend/src/markdown/safety.js)和[标题处理](../../frontend/src/markdown/headings.js)。[契约摘要](../../frontend/src/markdown/contract.json)与 Rendering 的 lib/markdown/ 对应文件保持一致；摘要修改须核对两仓同一批行为用例，不能仅刷新摘要跳过检查。

WSL Ubuntu 中进入 frontend 后，运行 npm run test:mdx-preview 检查解析契约和真实浏览器；运行 npm run test:article-images 验证受影响编辑功能，运行 npm run build 检查类型和打包。全量回归由 GitHub CI 执行。生产发布与真实页面验收按[当前任务](../plans/current-tasks.md)单独跟进。
