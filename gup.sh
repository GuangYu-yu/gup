#!/bin/bash

# 设置最大文件大小限制（100MB）
MAX_FILE_SIZE=$((100 * 1024 * 1024))

# 帮助信息
show_usage() {
    echo "用法: gup -f <文件路径> -u <GitHub URL>"
    echo "示例: gup -f ./example.txt -u https://raw.githubusercontent.com/用户名/仓库名/refs/heads/分支名/文件路径?token=访问令牌"
    exit 1
}

# 解析命令行参数
while [[ $# -gt 0 ]]; do
    case $1 in
        -f|--file)
            FILE_PATH="$2"
            shift 2
            ;;
        -u|--github-url)
            GITHUB_URL="$2"
            shift 2
            ;;
        *)
            show_usage
            ;;
    esac
done

# 检查必要参数
if [ -z "$FILE_PATH" ] || [ -z "$GITHUB_URL" ]; then
    echo "错误: 必须提供文件路径和 GitHub URL"
    show_usage
fi

# 检查文件是否存在
if [ ! -f "$FILE_PATH" ]; then
    echo "错误: 文件 $FILE_PATH 不存在"
    exit 1
fi

# 检查文件大小
FILE_SIZE=$(stat -f%z "$FILE_PATH" 2>/dev/null || stat -c%s "$FILE_PATH")
if [ $FILE_SIZE -gt $MAX_FILE_SIZE ]; then
    echo "错误: 文件大小 ($(echo "scale=2; $FILE_SIZE/1024/1024" | bc) MB) 超过限制 (100 MB)"
    exit 1
fi

# 解析 GitHub URL
if [[ $GITHUB_URL =~ ^https://raw\.githubusercontent\.com/([^/]+)/([^/]+)/refs/heads/([^/]+)/(.+)\?token=(.+)$ ]]; then
    USERNAME="${BASH_REMATCH[1]}"
    REPO_NAME="${BASH_REMATCH[2]}"
    BRANCH="${BASH_REMATCH[3]}"
    REMOTE_PATH="${BASH_REMATCH[4]}"
    TOKEN="${BASH_REMATCH[5]}"
    REPO="$USERNAME/$REPO_NAME"
    
    echo "解析 URL 成功:"
    echo "- 仓库: $REPO"
    echo "- 分支: $BRANCH"
    echo "- 路径: $REMOTE_PATH"
else
    echo "错误: GitHub URL 格式不正确"
    echo "正确格式: https://raw.githubusercontent.com/用户名/仓库名/refs/heads/分支名/文件路径?token=访问令牌"
    exit 1
fi

# base64 编码文件内容
ENCODED_CONTENT=$(base64 "$FILE_PATH" | tr -d '\n')

# 检查远程文件是否存在
API_URL="https://api.github.com/repos/$REPO/contents/$REMOTE_PATH"
CHECK_RESPONSE=$(curl -s -H "Authorization: token $TOKEN" \
    -H "Accept: application/vnd.github.v3+json" \
    -H "User-Agent: GitHub-Uploader-Shell/0.1.0" \
    "$API_URL")

if [ $? -ne 0 ]; then
    echo "发送请求失败"
    exit 1
fi

# 处理响应
if echo "$CHECK_RESPONSE" | grep -q '"sha"'; then
    # 文件存在，获取 SHA
    SHA=$(echo "$CHECK_RESPONSE" | grep -o '"sha":"[^"]*"' | cut -d'"' -f4)
    echo "远程文件已存在，将进行更新"
    FILE_EXISTS=true
else
    echo "远程文件不存在，将创建新文件"
    FILE_EXISTS=false
fi

# 准备请求数据
FILE_NAME=$(basename "$FILE_PATH")
if [ "$FILE_EXISTS" = true ]; then
    MESSAGE="更新 $FILE_NAME"
    JSON_DATA="{\"message\":\"$MESSAGE\",\"content\":\"$ENCODED_CONTENT\",\"branch\":\"$BRANCH\",\"sha\":\"$SHA\"}"
else
    MESSAGE="创建 $FILE_NAME"
    JSON_DATA="{\"message\":\"$MESSAGE\",\"content\":\"$ENCODED_CONTENT\",\"branch\":\"$BRANCH\"}"
fi

# 发送请求
RESPONSE=$(curl -s -X PUT "$API_URL" \
    -H "Authorization: token $TOKEN" \
    -H "Accept: application/vnd.github.v3+json" \
    -H "Content-Type: application/json" \
    -H "User-Agent: GitHub-Uploader-Shell/0.1.0" \
    -d "$JSON_DATA")

if [ $? -ne 0 ]; then
    echo "发送请求失败"
    exit 1
fi

# 检查响应
if echo "$RESPONSE" | grep -q '"commit"'; then
    COMMIT_SHA=$(echo "$RESPONSE" | grep -o '"sha":"[^"]*"' | head -1 | cut -d'"' -f4)
    echo "文件上传成功! Commit SHA: $COMMIT_SHA"
    exit 0
else
    echo "上传文件失败:"
    echo "$RESPONSE"
    exit 1
fi