## gocmt

你对 go 代码质量有更高的要求吗？那么注释一定是必不可少的。

`gocmt` 支持检查、补充、优化 go 代码注释，同时支持计算代码注释率，用于衡量项目是否有充足的注释说明。

## Installation

```bash
go install github.com/elliotxx/gocmt@latest
```

## Usage

```bash
$ gocmt -h
Usage: gocmt [options]

Options:
  -f  string
    File or directory containing Go code.
  -c  string
    Specify a commit hash or reference (e.g., HEAD, HEAD^, commitID1...commitID2)
  -n  int
    Number of concurrent executions
  -h  bool
    Show this help message and exit

Examples:
  gocmt -f /path/to/example.go
  gocmt -f /path/to/dir/
  gocmt -c HEAD
  gocmt -c HEAD^
  gocmt -c commitID1...commitID2
```

## Example

The go source code of no comment to be processed:

```bash
$ cat handler.go
package handler

func printDebugf(format string, args ...interface{}) {
}

type ErrorResponse struct {
}

func (e *ErrorResponse) String() string {
}

func Respond(w http.ResponseWriter, code int, src interface{}) {
}

func Error(w http.ResponseWriter, code int, err error, msg string) {
}

func JSON(w http.ResponseWriter, code int, src interface{}) {
}

type Handler struct {}

func (h Handler) Routes() *router.Router {
}

func (h Handler) Run(port int) error {
}

func (h Handler) getUser(w http.ResponseWriter, r *http.Request, id int) {
}

func (h Handler) getUsers(w http.ResponseWriter, r *http.Request) {
}

func (h Handler) createUser(w http.ResponseWriter, r *http.Request) {
}
```

Execute `gocmt`:

```bash
$ gocmt -f handler.go
» Comments will be added to these go files soon:
/Users/yym/tmp/test.go

» Processing /Users/yym/tmp/test.go...
✔ Processed file /Users/yym/tmp/test.go
Progress: 1/1, 100.00%

All files processed.
```

After processing with `gocmt`:

```bash
$ cat handler.go
package handler

// printDebugf printing debug information in a formatted manner.
func printDebugf(format string, args ...interface{}) {
}

type ErrorResponse struct {
}

// String returns the error message in string format.
func (e *ErrorResponse) String() string {
}

// Respond send a response with a given status code and source data.
func Respond(w http.ResponseWriter, code int, src interface{}) {
}

// Error send an error response with a given status code, error, and message.
func Error(w http.ResponseWriter, code int, err error, msg string) {
}

// JSON send a JSON response with a given status code and source data.
func JSON(w http.ResponseWriter, code int, src interface{}) {
}

type Handler struct {}

// Routes returns the router with all the routes configured.
func (h Handler) Routes() *router.Router {
}

// Run starts the HTTP server on the given port and listens for incoming requests.
func (h Handler) Run(port int) error {
}

// getUser handles the GET request for fetching a single user by ID.
func (h Handler) getUser(w http.ResponseWriter, r *http.Request, id int) {
}

// getUsers handles the GET request for fetching all the users.
func (h Handler) getUsers(w http.ResponseWriter, r *http.Request) {
}

// createUser handles the POST request for creating a new user.
func (h Handler) createUser(w http.ResponseWriter, r *http.Request) {
}
```

If you forgot to add comments in the latest git commit, you can do this:

```bash
$ gocmt -c HEAD^

# Or you can specify commit:
$ gocmt -c <commit-id>
# And this:
$ gocmt -c <commit-id-a>...<commid-id-b>
```

## TODO

-   [x] 通过 KIMI API 自动补充注释
-   [x] 文件或者目录作为输入
-   [x] 跳过已存在的注释
-   [x] 日志输出
-   [x] 输出进度条
-   [x] 忽略单元测试
-   [x] 并发执行
-   [x] 识别 git diff 对最近一次变更受影响的文件补充注释
-   [ ] 支持 --exclude 用来忽略指定目录
-   [ ] 失败重试机制
-   [ ] 支持 -f 指定多个路径
-   [ ] 执行前 token 消耗评估
-   [ ] 配置 maxLineWidth
-   [ ] 扫描不合规的注释
-   [ ] 扫描结果打标
-   [ ] 计算注释率
