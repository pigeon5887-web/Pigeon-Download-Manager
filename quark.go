package main

// 自建夸克解析器：NFD 这版对夸克只支持单文件（目录直接报"没有可下载的文件"），
// 本喵直接走夸克官方 API：token(提取码) → detail(目录树) → file/download(直链)
// 全程匿名，只要分享链接 + 提取码，不需要任何账号。
// 实测链路：2026-11-09 用真实分享验证 token/detail/download 均 200。

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

// quarkUA 必须和 addDownloadTask 里给 aria2 的 UA 一字不差：
// 夸克的直链按请求 UA 签发，两头不一致可能 403
const quarkUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

const quarkAPIBase = "https://drive-pc.quark.cn/1/clouddrive"

// 夸克专用 client：带 cookie jar，token/detail/download 的过程中
// 服务器会下发 .quark.cn 域的访客 cookie（__pugs/__sdid），
// CDN 下载直链时必须带上，不然 412（实测 A/B 过）
var (
	quarkJar, _ = cookiejar.New(nil)
	quarkClient = &http.Client{Timeout: 30 * time.Second, Jar: quarkJar}
)

type quarkResp struct {
	Status  int             `json:"status"`
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// quarkAPI 统一请求：自动带 UA/Referer，非 200 把夸克的原话报回来
func quarkAPI(method, apiURL, body string) (json.RawMessage, error) {
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
	req.Header.Set("User-Agent", quarkUA)
	req.Header.Set("Referer", "https://pan.quark.cn/")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := quarkClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var r quarkResp
	if json.Unmarshal(raw, &r) != nil {
		return nil, fmt.Errorf("夸克响应解析失败喵")
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

// quarkGetStoken 用分享ID+提取码换匿名访问令牌（谁的账号都不沾，纯访客门票）
func quarkGetStoken(pwdID, passcode string) (string, error) {
	data, err := quarkAPI("POST", quarkAPIBase+"/share/sharepage/token?pr=ucpro&fr=pc",
		fmt.Sprintf(`{"pwd_id":%q,"passcode":%q}`, pwdID, passcode))
	if err != nil {
		return "", err
	}
	var d struct {
		Stoken string `json:"stoken"`
	}
	if json.Unmarshal(data, &d) != nil || d.Stoken == "" {
		return "", fmt.Errorf("夸克没吐 stoken 喵")
	}
	return d.Stoken, nil
}

type quarkEntry struct {
	FID           string `json:"fid"`
	FileName      string `json:"file_name"`
	FileType      int    `json:"file_type"` // 0=文件夹 1=文件
	Size          int64  `json:"size"`
	ShareFidToken string `json:"share_fid_token"`
}

// quarkListDir 列目录（pdirFid="0" 是根），自动翻页
func quarkListDir(pwdID, stoken, pdirFid string) ([]quarkEntry, error) {
	var out []quarkEntry
	for page := 1; page <= 50; page++ {
		api := fmt.Sprintf("%s/share/sharepage/detail?pr=ucpro&fr=pc&pwd_id=%s&stoken=%s&pdir_fid=%s&_page=%d&_size=100&fetch_share=1",
			quarkAPIBase, url.QueryEscape(pwdID), url.QueryEscape(stoken), url.QueryEscape(pdirFid), page)
		data, err := quarkAPI("GET", api, "")
		if err != nil {
			return nil, err
		}
		var d struct {
			List []quarkEntry `json:"list"`
		}
		if json.Unmarshal(data, &d) != nil {
			return nil, fmt.Errorf("夸克目录解析失败喵")
		}
		out = append(out, d.List...)
		if len(d.List) == 0 {
			break
		}
	}
	return out, nil
}

// quarkToItems 转成通用 FileItem
// 文件夹带 SubRef（qkdir:pwdID|stoken|fid）供 cd 进入；
// 文件的 FileID（qk:pwdID|stoken|fid|shareFidToken）自带完整会话上下文供直链解析
func quarkToItems(pwdID, stoken string, entries []quarkEntry) []FileItem {
	items := make([]FileItem, 0, len(entries))
	for _, e := range entries {
		if e.FileType == 0 {
			items = append(items, FileItem{
				FileName: e.FileName,
				FileType: "folder",
				SubRef:   "qkdir:" + pwdID + "|" + stoken + "|" + e.FID,
			})
			continue
		}
		size := ""
		if e.Size > 0 {
			size = formatBytes(strconv.FormatInt(e.Size, 10))
		}
		items = append(items, FileItem{
			FileName: e.FileName,
			FileID:   "qk:" + pwdID + "|" + stoken + "|" + e.FID + "|" + e.ShareFidToken,
			FileType: "file",
			SizeStr:  size,
		})
	}
	return items
}

// quarkExtractPwdID 从 https://pan.quark.cn/s/xxx 里抠分享 ID
func quarkExtractPwdID(shareURL string) string {
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

// quarkFileList 解析夸克分享链接（根目录）
func quarkFileList(shareURL, passcode string) ([]FileItem, error) {
	pwdID := quarkExtractPwdID(shareURL)
	if pwdID == "" {
		return nil, fmt.Errorf("没从链接里认出夸克分享 ID 喵")
	}
	stoken, err := quarkGetStoken(pwdID, passcode)
	if err != nil {
		return nil, err
	}
	entries, err := quarkListDir(pwdID, stoken, "0")
	if err != nil {
		return nil, err
	}
	return quarkToItems(pwdID, stoken, entries), nil
}

// quarkResolveDownload 拿文件直链 + CDN 要验的访客 cookie
// file/download 必须带完整分享上下文（fids + share_fid_tokens + pwd_id + stoken），
// 缺了就 401 "share missing"（实测对照过）；
// 签发出来的直链必须带 .quark.cn 域的 __pugs/__sdid cookie 才能取，不然 412（实测 A/B 过）
func quarkResolveDownload(fileID string) (string, string, error) {
	rest := strings.TrimPrefix(fileID, "qk:")
	parts := strings.Split(rest, "|")
	if len(parts) != 4 {
		return "", "", fmt.Errorf("夸克文件标识格式不对喵")
	}
	pwdID, stoken, fid, tok := parts[0], parts[1], parts[2], parts[3]

	body := fmt.Sprintf(`{"fids":[%q],"share_fid_tokens":[%q],"pwd_id":%q,"stoken":%q}`,
		fid, tok, pwdID, stoken)
	data, err := quarkAPI("POST", quarkAPIBase+"/file/download?pr=ucpro&fr=pc", body)
	if err != nil {
		return "", "", err
	}
	var d []struct {
		DownloadURL string `json:"download_url"`
		FileName    string `json:"file_name"`
	}
	if json.Unmarshal(data, &d) != nil || len(d) == 0 {
		return "", "", fmt.Errorf("夸克直链响应异常喵")
	}
	if d[0].DownloadURL == "" {
		return "", "", fmt.Errorf("夸克没给直链喵（文件可能被限制下载）")
	}

	// 从 jar 里摘 .quark.cn 域的 cookie 给 aria2（cdn 会按域匹配自动带上 __pugs/__sdid）
	cookie := ""
	if u, perr := url.Parse(d[0].DownloadURL); perr == nil {
		var kv []string
		for _, c := range quarkJar.Cookies(u) {
			kv = append(kv, c.Name+"="+c.Value)
		}
		cookie = strings.Join(kv, "; ")
	}
	return d[0].DownloadURL, cookie, nil
}
