# GitHub 文件上传工具 (gup)

将文件上传到 GitHub 仓库。支持新文件创建和已有文件更新，自动处理文件冲突。

## 使用方法

基本用法：

```bash
gup -f <文件路径> -u <GitHub URL>
```

### 参数说明

- `-f, --file`: 要上传的文件路径
- `-u, --github-url`: GitHub 文件 URL，格式如下：
  ```
  https://raw.githubusercontent.com/用户名/仓库名/refs/heads/分支名/文件路径?token=访问令牌
  ```

### 使用示例

```bash
gup -f ./example.txt -u https://raw.githubusercontent.com/username/repo/refs/heads/main/docs/example.txt?token=ghp_xxxxxxxxxxxx
```
