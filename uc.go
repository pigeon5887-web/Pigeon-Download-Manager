package main

// 自建 UC 网盘解析器：NFD 这版对 UC 只支持单文件（根目录是文件夹就报"列表为空"），
// 本喵直接走 UC 官方 API：token(提取码) → transfer_share/detail(目录树) → file/download(直链)
// 全程匿名，只要分享链接 + 提取码，不需要任何账号。
// 实测链路：2026-11-09 用真实分享验证 token/detail/download 均 200，
// 直链 A/B：不带 __pugs/__sdid cookie 403，带上 206。

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ucUA 必须和 addDownloadTask 里给 aria2 的 UA 一字不差（直链按 UA 签发）
const ucUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

const (
	ucAPIBase    = "https://pc-api.uc.cn/1/clouddrive"
	ucReferer    = "https://fast.uc.cn/" // UC 的极速入口域；实测 drive/fast 两个 referer 均可，站 fast 讨个彩头
	ucDetailPath = "/transfer_share/detail" // UC 的目录端点和夸克不同族
)

// UC 专用 client：带 cookie jar，token/detail/download 过程中
// 服务器下发 .uc.cn 域的访客 cookie（__pugs/__sdid），CDN 直链必须带上（403→206 实测）
var (
	ucJar, _ = cookiejar.New(nil)
	ucClient = &http.Client{Timeout: 30 * time.Second, Jar: ucJar}
)

type ucResp struct {
	Status  int             `json:"status"`
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// ucAPI 统一请求：自动带 UA/Referer，非 200 把 UC 的原话报回来
func ucAPI(method, apiURL, body string) (json.RawMessage, error) {
	var req *http.Request
	var err error
	if body != "" {
		req, err = http.NewRequest(method, apiURL, strings.NewReader(body))
	} else {
		req, err = http.NewRequest(method, apiURL, nil)
	}
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", ucUA)
	req.Header.Set("Referer", ucReferer)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := ucClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var r ucResp
	if json.Unmarshal(raw, &r) != nil {
		return nil, fmt.Errorf("UC 响应解析失败喵")
	}
	if r.Status != 200 || r.Code != 0 {
		msg := r.Message
		if msg == "" {
			msg = fmt.Sprintf("code=%d", r.Code)
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return r.Data, nil
}

// ucGetStoken 用分享ID+提取码换匿名访问令牌（纯访客门票，不沾任何账号）
func ucGetStoken(pwdID, passcode string) (string, error) {
	data, err := ucAPI("POST", ucAPIBase+"/share/sharepage/token?pr=UCB&fr=pc",
		fmt.Sprintf(`{"pwd_id":%q,"passcode":%q}`, pwdID, passcode))
	if err != nil {
		return "", err
	}
	var d struct {
		Stoken string `json:"stoken"`
	}
	if json.Unmarshal(data, &d) != nil || d.Stoken == "" {
		return "", fmt.Errorf("UC 没吐 stoken 喵")
	}
	return d.Stoken, nil
}

type ucEntry struct {
	FID           string `json:"fid"`
	FileName      string `json:"file_name"`
	FileType      int    `json:"file_type"` // 0=文件夹 1=文件
	Size          int64  `json:"size"`
	ShareFidToken string `json:"share_fid_token"`
}

// ucListDir 列目录（pdirFid="0" 是根），自动翻页
func ucListDir(pwdID, stoken, pdirFid string) ([]ucEntry, error) {
	var out []ucEntry
	for page := 1; page <= 50; page++ {
		api := fmt.Sprintf("%s%s?pr=UCB&fr=pc&pwd_id=%s&stoken=%s&pdir_fid=%s&_page=%d&_size=100&fetch_share=1",
			ucAPIBase, ucDetailPath, url.QueryEscape(pwdID), url.QueryEscape(stoken), url.QueryEscape(pdirFid), page)
		data, err := ucAPI("GET", api, "")
		if err != nil {
			return nil, err
		}
		var d struct {
			List []ucEntry `json:"list"`
		}
		if json.Unmarshal(data, &d) != nil {
			return nil, fmt.Errorf("UC 目录解析失败喵")
		}
		out = append(out, d.List...)
		if len(d.List) == 0 {
			break
		}
	}
	return out, nil
}

// ucToItems 转成通用 FileItem
// 文件夹带 SubRef（ucdir:pwdID|stoken|fid）供 cd 进入；
// 文件的 FileID（uc:pwdID|stoken|fid|shareFidToken）自带完整会话上下文供直链解析
func ucToItems(pwdID, stoken string, entries []ucEntry) []FileItem {
	items := make([]FileItem, 0, len(entries))
	for _, e := range entries {
		if e.FileType == 0 {
			items = append(items, FileItem{
				FileName: e.FileName,
				FileType: "folder",
				SubRef:   "ucdir:" + pwdID + "|" + stoken + "|" + e.FID,
			})
			continue
		}
		size := ""
		if e.Size > 0 {
			size = formatBytes(strconv.FormatInt(e.Size, 10))
		}
		items = append(items, FileItem{
			FileName: e.FileName,
			FileID:   "uc:" + pwdID + "|" + stoken + "|" + e.FID + "|" + e.ShareFidToken,
			FileType: "file",
			SizeStr:  size,
		})
	}
	return items
}

// ucExtractPwdID 从 https://drive.uc.cn/s/xxx 或 https://fast.uc.cn/s/xxx 里抠分享 ID
func ucExtractPwdID(shareURL string) string {
	u, err := url.Parse(shareURL)
	if err != nil {
		return ""
	}
	p := strings.Trim(u.Path, "/")
	if strings.HasPrefix(p, "s/") {
		return strings.TrimPrefix(p, "s/")
	}
	return ""
}

// ucFileList 解析 UC 分享链接（根目录）
func ucFileList(shareURL, passcode string) ([]FileItem, error) {
	pwdID := ucExtractPwdID(shareURL)
	if pwdID == "" {
		return nil, fmt.Errorf("没从链接里认出 UC 分享 ID 喵")
	}
	stoken, err := ucGetStoken(pwdID, passcode)
	if err != nil {
		return nil, err
	}
	entries, err := ucListDir(pwdID, stoken, "0")
	if err != nil {
		return nil, err
	}
	return ucToItems(pwdID, stoken, entries), nil
}

// ucResolveDownload 拿文件直链 + CDN 要验的访客 cookie
// file/download 必须带完整分享上下文（fids + share_fid_tokens + pwd_id + stoken），
// 签发出来的直链必须带 .uc.cn 域的 __pugs/__sdid cookie 才能取（403→206 实测）
func ucResolveDownload(fileID string) (string, string, error) {
	rest := strings.TrimPrefix(fileID, "uc:")
	parts := strings.Split(rest, "|")
	if len(parts) != 4 {
		return "", "", fmt.Errorf("UC 文件标识格式不对喵")
	}
	pwdID, stoken, fid, tok := parts[0], parts[1], parts[2], parts[3]

	body := fmt.Sprintf(`{"fids":[%q],"share_fid_tokens":[%q],"pwd_id":%q,"stoken":%q}`,
		fid, tok, pwdID, stoken)
	data, err := ucAPI("POST", ucAPIBase+"/file/download?pr=UCB&fr=pc", body)
	if err != nil {
		return "", "", err
	}
	var d []struct {
		DownloadURL string `json:"download_url"`
		FileName    string `json:"file_name"`
	}
	if json.Unmarshal(data, &d) != nil || len(d) == 0 {
		return "", "", fmt.Errorf("UC 直链响应异常喵")
	}
	if d[0].DownloadURL == "" {
		return "", "", fmt.Errorf("UC 没给直链喵（文件可能被限制下载）")
	}

	// 从 jar 里摘 .uc.cn 域的 cookie 给 aria2（cdn 按域匹配自动带上 __pugs/__sdid）
	cookie := ""
	if u, perr := url.Parse(d[0].DownloadURL); perr == nil {
		var kv []string
		for _, c := range ucJar.Cookies(u) {
			kv = append(kv, c.Name+"="+c.Value)
		}
		cookie = strings.Join(kv, "; ")
	}
	return d[0].DownloadURL, cookie, nil
}
