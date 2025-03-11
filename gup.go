package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	maxFileSize = 100 * 1024 * 1024 // 100MB 限制
)

type GitHubResponse struct {
	SHA    string       `json:"sha,omitempty"`
	Commit *GitHubCommit `json:"commit,omitempty"`
}

type GitHubCommit struct {
	SHA string `json:"sha"`
}

func main() {
	// 检查命令行参数
	if len(os.Args) < 5 {
		fmt.Println("用法: gup -f <文件路径> -u <GitHub URL>")
		fmt.Println("示例: gup -f ./example.txt -u https://raw.githubusercontent.com/用户名/仓库名/refs/heads/分支名/文件路径?token=访问令牌")
		os.Exit(1)
	}

	var filePath, githubURL string

	// 解析命令行参数
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "-f" || os.Args[i] == "--file" {
			if i+1 < len(os.Args) {
				filePath = os.Args[i+1]
				i++
			}
		} else if os.Args[i] == "-u" || os.Args[i] == "--github-url" {
			if i+1 < len(os.Args) {
				githubURL = os.Args[i+1]
				i++
			}
		}
	}

	if filePath == "" || githubURL == "" {
		fmt.Println("错误: 必须提供文件路径和 GitHub URL")
		os.Exit(1)
	}

	success := uploadToGitHub(filePath, githubURL)
	if !success {
		os.Exit(1)
	}
}

func uploadToGitHub(filePath, githubURL string) bool {
	fmt.Printf("开始上传文件 %s 到 GitHub...\n", filePath)

	// 解析 GitHub URL
	re := regexp.MustCompile(`^https://raw\.githubusercontent\.com/([^/]+)/([^/]+)/refs/heads/([^/]+)/(.+)\?token=(.+)$`)
	matches := re.FindStringSubmatch(githubURL)
	if matches == nil {
		fmt.Println("错误: GitHub URL 格式不正确")
		fmt.Println("正确格式: https://raw.githubusercontent.com/用户名/仓库名/refs/heads/分支名/文件路径?token=访问令牌")
		return false
	}

	// 提取信息
	username := matches[1]
	repoName := matches[2]
	branch := matches[3]
	remotePath := matches[4]
	token := matches[5]
	repo := fmt.Sprintf("%s/%s", username, repoName)

	fmt.Println("解析 URL 成功:")
	fmt.Printf("- 仓库: %s\n", repo)
	fmt.Printf("- 分支: %s\n", branch)
	fmt.Printf("- 路径: %s\n", remotePath)

	// 检查文件是否存在
	fileInfo, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		fmt.Printf("错误: 文件 %s 不存在\n", filePath)
		return false
	}

	// 检查文件大小
	fileSize := fileInfo.Size()
	if fileSize > maxFileSize {
		fmt.Printf("错误: 文件大小 (%.2f MB) 超过限制 (100 MB)\n", float64(fileSize)/1024.0/1024.0)
		return false
	}

	// 读取文件内容并进行 base64 编码
	fileContent, err := ioutil.ReadFile(filePath)
	if err != nil {
		fmt.Printf("读取文件失败: %s\n", err)
		return false
	}
	encodedContent := base64.StdEncoding.EncodeToString(fileContent)

	// 创建 HTTP 客户端
	client := &http.Client{}

	// 检查远程文件是否存在
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/contents/%s", repo, remotePath)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		fmt.Printf("创建请求失败: %s\n", err)
		return false
	}

	req.Header.Add("Authorization", fmt.Sprintf("token %s", token))
	req.Header.Add("Accept", "application/vnd.github.v3+json")
	req.Header.Add("User-Agent", "GitHub-Uploader-Go/0.1.0")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("发送请求失败: %s\n", err)
		return false
	}
	defer resp.Body.Close()

	var fileExists bool
	var sha string

	if resp.StatusCode == 200 {
		// 文件存在，获取 SHA
		var response GitHubResponse
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("读取响应失败: %s\n", err)
			return false
		}

		err = json.Unmarshal(body, &response)
		if err != nil {
			fmt.Printf("解析响应失败: %s\n", err)
			return false
		}

		fmt.Println("远程文件已存在，将进行更新")
		fileExists = true
		sha = response.SHA
	} else if resp.StatusCode == 404 {
		// 文件不存在
		fmt.Println("远程文件不存在，将创建新文件")
		fileExists = false
	} else {
		fmt.Printf("检查远程文件失败: %d %s\n", resp.StatusCode, resp.Status)
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Printf("响应内容: %s\n", string(body))
		return false
	}

	// 准备请求数据
	fileName := filepath.Base(filePath)
	message := fmt.Sprintf("创建 %s", fileName)
	if fileExists {
		message = fmt.Sprintf("更新 %s", fileName)
	}

	type RequestData struct {
		Message string `json:"message"`
		Content string `json:"content"`
		Branch  string `json:"branch"`
		SHA     string `json:"sha,omitempty"`
	}

	requestData := RequestData{
		Message: message,
		Content: encodedContent,
		Branch:  branch,
	}

	if fileExists {
		requestData.SHA = sha
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		fmt.Printf("创建请求数据失败: %s\n", err)
		return false
	}

	// 发送请求
	req, err = http.NewRequest("PUT", apiURL, strings.NewReader(string(jsonData)))
	if err != nil {
		fmt.Printf("创建请求失败: %s\n", err)
		return false
	}

	req.Header.Add("Authorization", fmt.Sprintf("token %s", token))
	req.Header.Add("Accept", "application/vnd.github.v3+json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", "GitHub-Uploader-Go/0.1.0")

	resp, err = client.Do(req)
	if err != nil {
		fmt.Printf("发送请求失败: %s\n", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		var response GitHubResponse
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("读取响应失败: %s\n", err)
			return false
		}

		err = json.Unmarshal(body, &response)
		if err != nil {
			fmt.Printf("解析响应失败: %s\n", err)
			return false
		}

		if response.Commit != nil {
			fmt.Printf("文件上传成功! Commit SHA: %s\n", response.Commit.SHA)
		} else {
			fmt.Println("文件上传成功!")
		}
		return true
	} else {
		fmt.Printf("上传文件失败 (HTTP 错误): %d %s\n", resp.StatusCode, resp.Status)
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Printf("响应内容: %s\n", string(body))
		return false
	}
}