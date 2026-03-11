### 通用导入参数（适用于所有导入相关命令）

- `-language`: 指定文档的语言类型（留空表示自动推断）
  - 可选值：`golang|go`, `java`, `middleware|中间件`
- `-category`: 指定文档分类（留空表示自动推断）
  - 可选值：`basic|基础`, `specialized|专项`, `comprehensive|综合`
- `-source`: 元数据来源标记（字符串，默认 `cli`）
- `-recursive`: 目录导入是否递归（布尔，默认 `true`，仅目录导入生效）
# 单个命令
# 导入 Go 专项文档
```bash
go run . -cmd import-go专项 -file advanced-concurrency.md -language go -category specialized
```
# 导入 Go 基础文档
```bash
go run . -cmd import-go基础 -file intro.md -language go -category basic
```
# 导入 Java 专项文档
```bash
go run . -cmd import-java专项 -file jvm-tuning.md -language java -category specialized
```
# 导入 Java 基础文档
```bash
go run . -cmd import-java基础 -file intro.md -language java -category basic
```
# 导入 中间件专项文档
```bash
go run . -cmd import-中间件专项 -file kafka-deep-dive.md -language middleware -category specialized
```
# 导入 中间件基础文档
```bash
go run . -cmd import-中间件基础 -file intro.md -language middleware -category basic
```
# 导入 综合文档
```bash
go run . -cmd import-综合 -file architectures.md -category comprehensive
```

# 批量命令
# 批量导入 Go 专项文档
```bash
go run . -cmd batch-go专项 -language go -category specialized
```
# 批量导入 Go 基础文档
```bash
go run . -cmd batch-go基础 -language go -category basic
```
# 批量导入 Java 专项文档
```bash
go run . -cmd batch-java专项 -language java -category specialized
```
# 批量导入 Java 基础文档
```bash
go run . -cmd batch-java基础 -language java -category basic
```
# 批量导入 中间件专项文档
```bash
go run . -cmd batch-中间件专项 -language middleware -category specialized
```
# 批量导入 中间件基础文档
```bash
go run . -cmd batch-中间件基础 -language middleware -category basic
```
# 批量导入 综合文档
```bash
go run . -cmd batch-综合 -category comprehensive
```