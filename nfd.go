package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Parser interface {
	// GetFileList 用于获取目录或单文件信息
	GetFileList(shareURL, pwd string) ([]FileItem, error)
	// GetDirectLink 根据文件 ID 获取真实直链
	GetDirectLink(fileID string) (string, error)
}

type NfdParser struct {
	APIBase string
}

type FileItem struct {
	FileName string `json:"fileName"`
	FileID   string `json:"fileId"`
	SizeStr  string `json:"sizeStr"`
}

type nfdListResponse struct {
	Code int        `json:"code"`
	Msg  string     `json:"msg"`
	Data []FileItem `json:"data"`
}

// 补全了单文件响应的结构，获取真实文件名
type nfdSingleResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		DirectLink string `json:"directLink"`
		FileInfo   struct {
			FileName string `json:"fileName"`
		} `json:"fileInfo"`
	} `json:"data"`
}

// GetFileList 获取目录列表或单文件的信息
func (n *NfdParser) GetFileList(shareURL, pwd string) ([]FileItem, error) {
	// 蓝奏云目录
	if strings.Contains(shareURL, "lanzou") && strings.Contains(shareURL, "/b") {
		apiURL := fmt.Sprintf("%s/v2/getFileList?url=%s", n.APIBase, url.QueryEscape(shareURL))
		if pwd != "" {
			apiURL += "&pwd=" + url.QueryEscape(pwd)
		}

		resp, err := http.Get(apiURL)
		if err != nil {
			return nil, fmt.Errorf("主人请求列表失败了喵~: %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		var listResp nfdListResponse
		json.Unmarshal(body, &listResp)

		if listResp.Code != 200 {
			return nil, fmt.Errorf(listResp.Msg)
		}
		return listResp.Data, nil
	}

	// 单文件处理
	apiURL := fmt.Sprintf("%s/json/parser?url=%s", n.APIBase, url.QueryEscape(shareURL))
	if pwd != "" {
		apiURL += "&pwd=" + url.QueryEscape(pwd)
	}
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("主人~请求单文件失败喵: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var singleResp nfdSingleResponse
	json.Unmarshal(body, &singleResp)

	if singleResp.Code != 200 {
		return nil, fmt.Errorf(singleResp.Msg)
	}

	// 修复：将原本写死的 "单文件"，改成获取真实文件名
	realFileName := singleResp.Data.FileInfo.FileName
	if realFileName == "" {
		realFileName = "未知文件" // 兜底
	}

	// 如果是单文件，包装成只有一个元素的列表返回
	return []FileItem{{FileName: realFileName, FileID: shareURL, SizeStr: "未知"}}, nil
}

// GetDirectLink 获取直链
func (n *NfdParser) GetDirectLink(fileID string) (string, error) {
	// 如果是单文件（我们上面把原链接塞到了 FileID 里）
	var apiURL string
	if strings.HasPrefix(fileID, "http") {
		apiURL = fmt.Sprintf("%s/json/parser?url=%s", n.APIBase, url.QueryEscape(fileID))
	} else {
		// 如果是目录里的文件
		apiURL = fmt.Sprintf("%s/json/lz/%s", n.APIBase, url.PathEscape(fileID))
	}

	resp, err := http.Get(apiURL)
	if err != nil {
		return "", fmt.Errorf("呜呜呜,请求直链失败喵: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var singleResp nfdSingleResponse
	json.Unmarshal(body, &singleResp)

	if singleResp.Code != 200 {
		return "", fmt.Errorf(singleResp.Msg)
	}
	return singleResp.Data.DirectLink, nil
}
