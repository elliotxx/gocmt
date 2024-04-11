## gocmt

你对 go 代码质量有更高的要求吗？那么注释一定是必不可少的。

`gocmt` 支持检查、补充、优化 go 代码注释，同时支持计算代码注释率，用于衡量项目是否有充足的注释说明。

## Installation

```bash
go install github.com/elliotxx/gocmt
```

## Usage

```bash
gocmt -f example.go
gocmt -f path/to/foo/
```

## TODO

-   [x] 通过 KIMI API 自动补充注释
-   [x] 文件或者目录作为输入
-   [x] 跳过已存在的注释
-   [x] 日志输出
-   [x] 输出进度条
-   [x] 忽略单元测试
-   [ ] 执行前 token 消耗评估
-   [ ] 配置 maxLineWidth
-   [ ] 扫描不合规的注释
-   [ ] 扫描结果打标
-   [ ] 计算注释率
